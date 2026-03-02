package providers

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ProviderRegistry holds configured Provider instances keyed by their ID.
// It is distinct from the package-level factory registry (Register/New).
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewProviderRegistry creates an empty provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

// Add registers a provider instance using p.ID() as the key.
// Panics if a provider with the same ID is already registered.
func (r *ProviderRegistry) Add(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := p.ID()
	if _, exists := r.providers[id]; exists {
		panic(fmt.Sprintf("provider already registered: %q", id))
	}
	r.providers[id] = p
}

// Get returns the provider for the given ID, or an error if not found.
func (r *ProviderRegistry) Get(id string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("no configured provider %q", id)
	}
	return p, nil
}

// Resolve parses a "providerId/modelId" string, looks up the provider,
// and returns the provider and bare model ID.
func (r *ProviderRegistry) Resolve(modelRef string) (Provider, string, error) {
	ref, err := ParseModelRef(modelRef)
	if err != nil {
		return nil, "", err
	}
	p, err := r.Get(ref.ProviderID)
	if err != nil {
		return nil, "", err
	}
	return p, ref.ModelID, nil
}

// ListModels queries all registered providers and returns their models
// with IDs prefixed as "providerId/modelId".
func (r *ProviderRegistry) ListModels(ctx context.Context) ([]ModelInfo, error) {
	r.mu.RLock()
	// Copy the map so we can release the lock during network calls.
	provs := make(map[string]Provider, len(r.providers))
	for id, p := range r.providers {
		provs[id] = p
	}
	r.mu.RUnlock()

	// Iterate in sorted order for deterministic results.
	ids := make([]string, 0, len(provs))
	for id := range provs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var all []ModelInfo
	for _, id := range ids {
		models, err := provs[id].ListModels(ctx)
		if err != nil {
			return nil, fmt.Errorf("list models from %s: %w", id, err)
		}
		for _, m := range models {
			m.ID = id + "/" + m.ID
			m.ProviderID = id
			all = append(all, m)
		}
	}
	return all, nil
}

// IDs returns the sorted list of registered provider IDs.
func (r *ProviderRegistry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
