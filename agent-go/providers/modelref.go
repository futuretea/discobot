package providers

import (
	"fmt"
	"strings"
)

// ModelRef is a parsed "providerId/modelId" string.
type ModelRef struct {
	ProviderID string
	ModelID    string
}

// ParseModelRef splits a "providerId/modelId" string on the first "/".
// Returns an error if the format is invalid.
func ParseModelRef(ref string) (ModelRef, error) {
	i := strings.IndexByte(ref, '/')
	if i < 0 {
		return ModelRef{}, fmt.Errorf("invalid model ref %q: expected providerId/modelId", ref)
	}
	providerID := ref[:i]
	modelID := ref[i+1:]
	if providerID == "" {
		return ModelRef{}, fmt.Errorf("invalid model ref %q: empty provider ID", ref)
	}
	if modelID == "" {
		return ModelRef{}, fmt.Errorf("invalid model ref %q: empty model ID", ref)
	}
	return ModelRef{ProviderID: providerID, ModelID: modelID}, nil
}

// String returns the "providerId/modelId" form.
func (r ModelRef) String() string {
	return r.ProviderID + "/" + r.ModelID
}
