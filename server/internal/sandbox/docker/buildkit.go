// Package docker provides BuildKit container management for shared Docker build caching.
// Each project gets one BuildKit container that acts as a remote builder,
// sharing build cache across all session containers within the project.
package docker

import (
	"context"
	"fmt"
	"log"

	cerrdefs "github.com/containerd/errdefs"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
)

const (
	// buildkitContainerPrefix is the prefix for per-project BuildKit container names.
	buildkitContainerPrefix = "discobot-buildkit-"

	// buildkitNetworkPrefix is the prefix for per-project Docker networks.
	buildkitNetworkPrefix = "discobot-net-"

	// buildkitPort is the gRPC port that buildkitd listens on.
	buildkitPort = 1234
)

// buildkitContainerName generates a BuildKit container name from project ID.
func buildkitContainerName(projectID string) string {
	return fmt.Sprintf("%s%s", buildkitContainerPrefix, projectID)
}

// buildkitNetworkName generates a project network name from project ID.
func buildkitNetworkName(projectID string) string {
	return fmt.Sprintf("%s%s", buildkitNetworkPrefix, projectID)
}

// EnsureBuildKit ensures a BuildKit container and project network exist and are
// running for the given project. This is idempotent — if the container already
// exists and is running with the correct image, this is a no-op.
// Returns the container name (for DNS resolution within the project network).
func (p *Provider) EnsureBuildKit(ctx context.Context, projectID string) (string, error) {
	name := buildkitContainerName(projectID)
	expectedImage := p.cfg.SandboxImage

	// Check if container already exists
	info, err := p.client.ContainerInspect(ctx, name)
	if err == nil {
		// Container exists — check if it's using the correct image and running
		if info.Config.Image == expectedImage && info.State.Running {
			return name, nil
		}
		if info.Config.Image == expectedImage && !info.State.Running {
			// Right image but stopped — start it
			if startErr := p.client.ContainerStart(ctx, info.ID, containerTypes.StartOptions{}); startErr != nil {
				return "", fmt.Errorf("failed to start buildkit container: %w", startErr)
			}
			log.Printf("Started existing BuildKit container %s for project %s", name, projectID)
			return name, nil
		}
		// Wrong image — remove and recreate
		log.Printf("BuildKit container %s uses outdated image %s (expected %s), recreating",
			name, info.Config.Image, expectedImage)
		if removeErr := p.client.ContainerRemove(ctx, info.ID, containerTypes.RemoveOptions{Force: true}); removeErr != nil {
			return "", fmt.Errorf("failed to remove outdated buildkit container: %w", removeErr)
		}
	}

	// Ensure the project network exists
	networkName, err := p.ensureProjectNetwork(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure project network: %w", err)
	}

	// Ensure cache volume exists for BuildKit state persistence
	cacheVolName, err := p.ensureCacheVolume(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure cache volume for buildkit: %w", err)
	}

	// Wait for the sandbox image to be available (same image used for BuildKit)
	if err := p.EnsureImage(ctx); err != nil {
		return "", fmt.Errorf("failed to ensure sandbox image for buildkit: %w", err)
	}

	containerConfig := &containerTypes.Config{
		Image: expectedImage,
		Cmd: []string{
			"buildkitd",
			"--addr", fmt.Sprintf("tcp://0.0.0.0:%d", buildkitPort),
		},
		Labels: map[string]string{
			"discobot.managed":    "true",
			"discobot.type":       "buildkit",
			"discobot.project.id": projectID,
		},
	}

	hostConfig := &containerTypes.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: cacheVolName,
				Target: "/var/lib/buildkit",
			},
		},
		// BuildKit needs privileges for container execution
		Privileged: true,
		// Auto-restart the BuildKit container if it crashes
		RestartPolicy: containerTypes.RestartPolicy{
			Name: containerTypes.RestartPolicyUnlessStopped,
		},
	}
	hostConfig.Ulimits = []*containerTypes.Ulimit{{
		Name: "nofile",
		Soft: 1048576,
		Hard: 1048576,
	}}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	resp, err := p.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create buildkit container: %w", err)
	}

	if err := p.client.ContainerStart(ctx, resp.ID, containerTypes.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start buildkit container: %w", err)
	}

	log.Printf("BuildKit container %s created and started for project %s (image: %s)", name, projectID, expectedImage)
	return name, nil
}

// ensureProjectNetwork creates a Docker bridge network for the project if it doesn't exist.
// This network connects session containers to the BuildKit container for the same project.
func (p *Provider) ensureProjectNetwork(ctx context.Context, projectID string) (string, error) {
	networkName := buildkitNetworkName(projectID)

	// Check if network already exists
	if _, err := p.client.NetworkInspect(ctx, networkName, network.InspectOptions{}); err == nil {
		return networkName, nil
	}

	// Create the network
	if _, err := p.client.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			"discobot.managed":    "true",
			"discobot.type":       "network",
			"discobot.project.id": projectID,
		},
	}); err != nil {
		return "", fmt.Errorf("failed to create project network: %w", err)
	}

	log.Printf("Created project network %s for project %s", networkName, projectID)
	return networkName, nil
}

// RemoveBuildKit removes the BuildKit container and project network for a project.
// This should be called when a project is deleted.
func (p *Provider) RemoveBuildKit(ctx context.Context, projectID string) error {
	name := buildkitContainerName(projectID)

	// Remove the BuildKit container
	if err := p.client.ContainerRemove(ctx, name, containerTypes.RemoveOptions{Force: true}); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return fmt.Errorf("failed to remove buildkit container: %w", err)
		}
	}

	// Remove the project network
	networkName := buildkitNetworkName(projectID)
	if err := p.client.NetworkRemove(ctx, networkName); err != nil {
		if !cerrdefs.IsNotFound(err) {
			log.Printf("Warning: failed to remove project network %s: %v", networkName, err)
		}
	}

	return nil
}

// ReconcileBuildKit checks all BuildKit containers and removes any that use
// an outdated image. They will be recreated with the correct image on the
// next session start for their project.
func (p *Provider) ReconcileBuildKit(ctx context.Context) error {
	expectedImage := p.cfg.SandboxImage

	containers, err := p.client.ContainerList(ctx, containerTypes.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "discobot.managed=true"),
			filters.Arg("label", "discobot.type=buildkit"),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to list buildkit containers: %w", err)
	}

	for _, c := range containers {
		projectID := c.Labels["discobot.project.id"]
		if projectID == "" {
			continue
		}

		if c.Image == expectedImage {
			log.Printf("BuildKit container for project %s uses correct image", projectID)
			continue
		}

		log.Printf("BuildKit container for project %s uses outdated image %s (expected %s), removing",
			projectID, c.Image, expectedImage)
		if removeErr := p.client.ContainerRemove(ctx, c.ID, containerTypes.RemoveOptions{Force: true}); removeErr != nil {
			log.Printf("Warning: failed to remove outdated BuildKit container for project %s: %v", projectID, removeErr)
		}
	}

	return nil
}
