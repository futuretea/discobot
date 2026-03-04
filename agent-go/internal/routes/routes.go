// Package routes provides a centralized route registry with metadata.
// Routes are registered with both chi router and a metadata store,
// keeping route definitions and documentation in one place.
package routes

import (
	"net/http"
	"regexp"
	"sync"

	"github.com/go-chi/chi/v5"
)

// Route defines an HTTP route with its handler and metadata.
type Route struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
	Meta    Meta
}

// Meta contains route documentation and metadata.
type Meta struct {
	Group       string  `json:"group"`
	Description string  `json:"description"`
	Params      []Param `json:"params,omitempty"`
	Body        any     `json:"body,omitempty"`
}

// Param describes a route parameter.
type Param struct {
	Name     string `json:"name"`
	In       string `json:"in"` // "path", "query"
	Required bool   `json:"required,omitempty"`
	Example  string `json:"example,omitempty"`
}

// RouteInfo is the JSON output format for the /api/routes endpoint.
type RouteInfo struct {
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Group       string  `json:"group"`
	Description string  `json:"description"`
	Params      []Param `json:"params,omitempty"`
	Body        any     `json:"body,omitempty"`
}

// Registry stores route metadata for documentation.
type Registry struct {
	mu     sync.RWMutex
	routes *[]RouteInfo // pointer to shared slice
	prefix string
}

// NewRegistry creates a new route registry.
func NewRegistry() *Registry {
	routes := make([]RouteInfo, 0)
	return &Registry{
		routes: &routes,
	}
}

// pathParamRegex matches chi path parameters like {threadId}
var pathParamRegex = regexp.MustCompile(`\{([^}]+)\}`)

// Register adds a route to chi and stores its metadata.
func (reg *Registry) Register(r chi.Router, route Route) {
	// Register with chi
	switch route.Method {
	case "GET":
		r.Get(route.Pattern, route.Handler)
	case "POST":
		r.Post(route.Pattern, route.Handler)
	case "PUT":
		r.Put(route.Pattern, route.Handler)
	case "DELETE":
		r.Delete(route.Pattern, route.Handler)
	case "PATCH":
		r.Patch(route.Pattern, route.Handler)
	}

	// Build full path
	fullPath := reg.prefix + route.Pattern

	// Extract path parameters from pattern
	params := extractPathParams(fullPath)

	// Add any additional params from Meta (query params, overrides)
	params = mergeParams(params, route.Meta.Params)

	// Store metadata
	reg.mu.Lock()
	*reg.routes = append(*reg.routes, RouteInfo{
		Method:      route.Method,
		Path:        fullPath,
		Group:       route.Meta.Group,
		Description: route.Meta.Description,
		Params:      params,
		Body:        route.Meta.Body,
	})
	reg.mu.Unlock()
}

// Group creates a sub-registry with a path prefix for nested routes.
func (reg *Registry) Group(pattern string) *Registry {
	return &Registry{
		routes: reg.routes, // share the same slice via pointer
		prefix: reg.prefix + pattern,
	}
}

// WithPrefix returns a new registry that shares storage but adds a prefix.
func (reg *Registry) WithPrefix(pattern string) *Registry {
	return &Registry{
		prefix: reg.prefix + pattern,
		routes: reg.routes, // share the same slice via pointer
	}
}

// Routes returns all registered route metadata.
func (reg *Registry) Routes() []RouteInfo {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	result := make([]RouteInfo, len(*reg.routes))
	copy(result, *reg.routes)
	return result
}

// extractPathParams extracts path parameters from a chi pattern.
func extractPathParams(pattern string) []Param {
	matches := pathParamRegex.FindAllStringSubmatch(pattern, -1)
	params := make([]Param, 0, len(matches))
	for _, match := range matches {
		params = append(params, Param{
			Name:     match[1],
			In:       "path",
			Required: true,
		})
	}
	return params
}

// mergeParams combines extracted path params with additional params from Meta.
func mergeParams(pathParams, metaParams []Param) []Param {
	if len(metaParams) == 0 {
		return pathParams
	}

	metaMap := make(map[string]Param)
	for _, p := range metaParams {
		metaMap[p.Name] = p
	}

	result := make([]Param, 0, len(pathParams)+len(metaParams))
	seen := make(map[string]bool)

	for _, p := range pathParams {
		if override, ok := metaMap[p.Name]; ok {
			override.In = "path"
			override.Required = true
			result = append(result, override)
		} else {
			result = append(result, p)
		}
		seen[p.Name] = true
	}

	for _, p := range metaParams {
		if !seen[p.Name] {
			result = append(result, p)
		}
	}

	return result
}

// Global registry instance.
var globalRegistry = NewRegistry()

// Register adds a route using the global registry.
func Register(r chi.Router, route Route) {
	globalRegistry.Register(r, route)
}

// All returns all routes from the global registry.
func All() []RouteInfo {
	return globalRegistry.Routes()
}

// GetRegistry returns the global registry.
func GetRegistry() *Registry {
	return globalRegistry
}
