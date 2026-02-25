//go:build darwin

package vz

import (
	"context"
	"fmt"
	"log"
	"time"

	containerTypes "github.com/docker/docker/api/types/container"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox/docker"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

// NewProvider creates a new VZ+Docker hybrid provider.
// It creates a VZ VMManager (which handles async image download if needed)
// and returns a generic vm.Provider that uses it for VM management.
func NewProvider(cfg *config.Config, vmConfig *vm.Config, resolver vm.SessionProjectResolver, systemManager vm.SystemManager) (*vm.Provider, error) {
	vmManager, err := NewVMManager(*vmConfig, systemManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create VZ VM manager: %w", err)
	}

	sandboxImage := cfg.SandboxImage

	opts := []vm.Option{
		vm.WithPostVMSetup(func(ctx context.Context, projectID string, dockerProv *docker.Provider) error {
			return startProxyContainer(ctx, projectID, dockerProv, sandboxImage)
		}),
	}

	// Parse idle timeout from VM config
	if vmConfig.IdleTimeout != "" {
		idleTimeout, err := time.ParseDuration(vmConfig.IdleTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid idle timeout %q: %w", vmConfig.IdleTimeout, err)
		}
		if idleTimeout > 0 {
			opts = append(opts, vm.WithIdleTimeout(idleTimeout))
		}
	}

	return vm.NewProvider(cfg, vmManager, resolver, systemManager, opts...), nil
}

// startProxyContainer creates and starts the VSOCK port proxy container inside the VM.
// The proxy watches Docker events for containers with published ports and creates
// socat VSOCK listeners to forward those ports to the host.
func startProxyContainer(ctx context.Context, projectID string, dockerProv *docker.Provider, sandboxImage string) error {
	cli := dockerProv.Client()
	suffix := projectID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	name := fmt.Sprintf("discobot-proxy-%s", suffix)

	// Check if proxy container already exists
	existing, err := cli.ContainerInspect(ctx, name)
	if err == nil {
		// Recreate if image changed or not privileged
		needsRecreate := existing.Config.Image != sandboxImage ||
			!existing.HostConfig.Privileged

		if existing.State.Running && !needsRecreate {
			log.Printf("Proxy container %s already running for project %s", name, projectID)
			return nil
		}
		if needsRecreate {
			log.Printf("Proxy container %s has stale config, recreating", name)
		}
		_ = cli.ContainerRemove(ctx, existing.ID, containerTypes.RemoveOptions{Force: true})
	}

	// Wait for the sandbox image to be available (pulled on provider startup).
	if err := dockerProv.EnsureImage(ctx); err != nil {
		return fmt.Errorf("failed to ensure sandbox image: %w", err)
	}

	containerConfig := &containerTypes.Config{
		Image: sandboxImage,
		Cmd:   []string{"/opt/discobot/bin/discobot-agent", "proxy"},
		Labels: map[string]string{
			"discobot.proxy":      "true",
			"discobot.project.id": projectID,
		},
	}

	hostConfig := &containerTypes.HostConfig{
		NetworkMode: "host",
		IpcMode:     "host",
		Privileged:  true, // Required for /dev/vsock access
		Binds:       []string{"/var/run/docker.sock:/var/run/docker.sock"},
		RestartPolicy: containerTypes.RestartPolicy{
			Name: containerTypes.RestartPolicyAlways,
		},
	}
	hostConfig.Ulimits = []*containerTypes.Ulimit{{
		Name: "nofile",
		Soft: 1048576,
		Hard: 1048576,
	}}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return fmt.Errorf("failed to create proxy container: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, containerTypes.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start proxy container: %w", err)
	}

	log.Printf("Started proxy container %s (%s) for project %s", name, resp.ID[:12], projectID)
	return nil
}
