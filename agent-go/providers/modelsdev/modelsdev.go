// Package modelsdev provides access to embedded models.dev metadata.
// It loads model metadata (context window, max output tokens, reasoning
// support, display name) from the bundled models-dev-api.json file.
//
// Providers call Lookup(providerID, modelID) to enrich their model lists
// with data from models.dev rather than hardcoding it.
package modelsdev

import (
	"embed"
	"encoding/json"
	"log"
	"sync"
)

//go:embed models-dev-api.json
var fs embed.FS

// ModelInfo contains the metadata for a single model from models.dev.
type ModelInfo struct {
	ID              string
	Name            string
	Family          string
	Reasoning       bool
	ContextWindow   int
	MaxOutputTokens int
}

type modelsDevData map[string]providerEntry

type providerEntry struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	API    string                   `json:"api"` // default base URL
	Env    []string                 `json:"env"` // required env var names (e.g. ["ANTHROPIC_API_KEY"])
	Models map[string]modelMetadata `json:"models"`
}

type modelMetadata struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Family    string     `json:"family"`
	Reasoning bool       `json:"reasoning"`
	Limit     modelLimit `json:"limit"`
}

type modelLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

var (
	once    sync.Once
	data    modelsDevData
	loadErr error
)

func load() {
	once.Do(func() {
		raw, err := fs.ReadFile("models-dev-api.json")
		if err != nil {
			log.Printf("modelsdev: failed to read embedded data: %v", err)
			loadErr = err
			return
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			log.Printf("modelsdev: failed to parse embedded data: %v", err)
			loadErr = err
		}
	})
}

// ProviderInfo holds provider-level metadata from models.dev.
type ProviderInfo struct {
	ID      string
	Name    string
	API     string   // default base URL
	EnvVars []string // required env var names (e.g. ["ANTHROPIC_API_KEY"])
}

// LookupProvider returns provider-level metadata for the given provider ID.
// Returns nil if the provider is not found.
func LookupProvider(providerID string) *ProviderInfo {
	load()
	if loadErr != nil {
		return nil
	}
	p, ok := data[providerID]
	if !ok {
		return nil
	}
	return &ProviderInfo{
		ID:      p.ID,
		Name:    p.Name,
		API:     p.API,
		EnvVars: p.Env,
	}
}

// Lookup returns model metadata for a specific provider and model ID.
// Returns nil if the provider or model is not found.
func Lookup(providerID, modelID string) *ModelInfo {
	load()
	if loadErr != nil {
		return nil
	}
	provider, ok := data[providerID]
	if !ok {
		return nil
	}
	m, ok := provider.Models[modelID]
	if !ok {
		return nil
	}
	return &ModelInfo{
		ID:              m.ID,
		Name:            m.Name,
		Family:          m.Family,
		Reasoning:       m.Reasoning,
		ContextWindow:   m.Limit.Context,
		MaxOutputTokens: m.Limit.Output,
	}
}

// AllForProvider returns all model metadata for a given provider ID.
// Returns nil if the provider is not found.
func AllForProvider(providerID string) []ModelInfo {
	load()
	if loadErr != nil {
		return nil
	}
	provider, ok := data[providerID]
	if !ok {
		return nil
	}
	models := make([]ModelInfo, 0, len(provider.Models))
	for _, m := range provider.Models {
		models = append(models, ModelInfo{
			ID:              m.ID,
			Name:            m.Name,
			Family:          m.Family,
			Reasoning:       m.Reasoning,
			ContextWindow:   m.Limit.Context,
			MaxOutputTokens: m.Limit.Output,
		})
	}
	return models
}
