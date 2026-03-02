package providers

import (
	"fmt"
	"sort"
	"sync"
)

// ProviderFactory creates a Provider instance from configuration.
// This is what gets registered in the registry, not a Provider instance,
// because providers need credentials to be constructed.
type ProviderFactory func(cfg Config) (Provider, error)

var (
	mu        sync.RWMutex
	factories = make(map[string]ProviderFactory)
)

// Register registers a provider factory under the given ID.
// It panics if a factory is already registered for that ID.
// This is typically called from init() functions.
func Register(id string, factory ProviderFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := factories[id]; exists {
		panic(fmt.Sprintf("providers: factory already registered for %q", id))
	}
	factories[id] = factory
}

// New creates a new Provider instance for the given provider ID
// using the supplied configuration. It returns an error if no factory
// is registered for the ID.
func New(id string, cfg Config) (Provider, error) {
	mu.RLock()
	factory, ok := factories[id]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("providers: unknown provider %q", id)
	}
	return factory(cfg)
}

// Has reports whether a factory is registered for the given provider ID.
func Has(id string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := factories[id]
	return ok
}

// RegisteredIDs returns the sorted list of all registered provider IDs.
func RegisteredIDs() []string {
	mu.RLock()
	defer mu.RUnlock()
	ids := make([]string, 0, len(factories))
	for id := range factories {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
