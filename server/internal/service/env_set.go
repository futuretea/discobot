package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/encryption"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

var (
	ErrEnvSetNotFound = errors.New("env set not found")
)

// EnvSetInfo is the client-safe representation of an env set (no secrets).
type EnvSetInfo struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// EnvSetWithVars includes the decrypted env vars (used for GET/{id}, POST, PUT responses).
type EnvSetWithVars struct {
	EnvSetInfo
	EnvVars map[string]string `json:"envVars"`
}

// EnvSetService manages env sets with AES encryption for values.
type EnvSetService struct {
	store     *store.Store
	encryptor *encryption.Encryptor
}

// NewEnvSetService creates a new EnvSetService.
func NewEnvSetService(s *store.Store, cfg *config.Config) (*EnvSetService, error) {
	enc, err := encryption.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor for env set service: %w", err)
	}
	return &EnvSetService{store: s, encryptor: enc}, nil
}

func toEnvSetInfo(e *model.EnvSet) EnvSetInfo {
	return EnvSetInfo{
		ID:        e.ID,
		ProjectID: e.ProjectID,
		Name:      e.Name,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

// List returns metadata for all env sets in a project (no secrets).
func (s *EnvSetService) List(ctx context.Context, projectID string) ([]EnvSetInfo, error) {
	envSets, err := s.store.ListEnvSetsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]EnvSetInfo, len(envSets))
	for i, e := range envSets {
		result[i] = toEnvSetInfo(e)
	}
	return result, nil
}

// Get returns a single env set with its decrypted env vars.
func (s *EnvSetService) Get(ctx context.Context, projectID, id string) (*EnvSetWithVars, error) {
	e, err := s.store.GetEnvSetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrEnvSetNotFound
		}
		return nil, err
	}
	if e.ProjectID != projectID {
		return nil, ErrEnvSetNotFound
	}
	envVars, err := s.decrypt(e)
	if err != nil {
		return nil, err
	}
	return &EnvSetWithVars{EnvSetInfo: toEnvSetInfo(e), EnvVars: envVars}, nil
}

// Create creates a new env set with the given name and env vars.
func (s *EnvSetService) Create(ctx context.Context, projectID, name string, envVars map[string]string) (*EnvSetWithVars, error) {
	encrypted, err := s.encryptor.EncryptJSON(envVars)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt env vars: %w", err)
	}
	e := &model.EnvSet{
		ProjectID:     projectID,
		Name:          name,
		EncryptedData: encrypted,
	}
	if err := s.store.CreateEnvSet(ctx, e); err != nil {
		return nil, err
	}
	return &EnvSetWithVars{EnvSetInfo: toEnvSetInfo(e), EnvVars: envVars}, nil
}

// Update updates an existing env set's name and/or env vars.
func (s *EnvSetService) Update(ctx context.Context, projectID, id, name string, envVars map[string]string) (*EnvSetWithVars, error) {
	e, err := s.store.GetEnvSetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrEnvSetNotFound
		}
		return nil, err
	}
	if e.ProjectID != projectID {
		return nil, ErrEnvSetNotFound
	}

	encrypted, err := s.encryptor.EncryptJSON(envVars)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt env vars: %w", err)
	}
	e.Name = name
	e.EncryptedData = encrypted

	if err := s.store.UpdateEnvSet(ctx, e); err != nil {
		return nil, err
	}
	return &EnvSetWithVars{EnvSetInfo: toEnvSetInfo(e), EnvVars: envVars}, nil
}

// Delete removes an env set by ID.
func (s *EnvSetService) Delete(ctx context.Context, projectID, id string) error {
	e, err := s.store.GetEnvSetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrEnvSetNotFound
		}
		return err
	}
	if e.ProjectID != projectID {
		return ErrEnvSetNotFound
	}
	return s.store.DeleteEnvSet(ctx, id)
}

// SetSessionActiveEnvSets sets the active env sets for a session.
// Ownership of the provided IDs must be validated before calling this method
// (see middleware.EnvSetsOwnedByProject for the HTTP path).
// Pass an empty slice to clear all active env sets.
func (s *EnvSetService) SetSessionActiveEnvSets(ctx context.Context, sessionID string, envSetIDs []string) error {
	return s.store.UpdateSessionActiveEnvSets(ctx, sessionID, envSetIDs)
}

// GetEnvVarsForSession returns the merged decrypted env vars from all active env sets.
// Sets are merged in order; later sets override earlier ones on key conflicts.
// Returns an empty map if no env sets are active or all active sets are missing.
func (s *EnvSetService) GetEnvVarsForSession(ctx context.Context, sessionID string) (map[string]string, error) {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if len(sess.ActiveEnvSetIDs) == 0 {
		return map[string]string{}, nil
	}

	merged := map[string]string{}
	for _, id := range sess.ActiveEnvSetIDs {
		e, err := s.store.GetEnvSetByID(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// Env set was deleted — skip it
				continue
			}
			return nil, err
		}
		// Defense-in-depth: skip env sets that don't belong to this session's project.
		// This guards against IDs that may have been stored before ownership was validated.
		if e.ProjectID != sess.ProjectID {
			log.Printf("Warning: env set %s does not belong to project %s (session %s), skipping", id, sess.ProjectID, sessionID)
			continue
		}
		vars, err := s.decrypt(e)
		if err != nil {
			return nil, err
		}
		for k, v := range vars {
			merged[k] = v
		}
	}

	return merged, nil
}

func (s *EnvSetService) decrypt(e *model.EnvSet) (map[string]string, error) {
	if len(e.EncryptedData) == 0 {
		return map[string]string{}, nil
	}
	var envVars map[string]string
	if err := s.encryptor.DecryptJSON(e.EncryptedData, &envVars); err != nil {
		return nil, fmt.Errorf("failed to decrypt env vars for env set %s: %w", e.ID, err)
	}
	if envVars == nil {
		envVars = map[string]string{}
	}
	return envVars, nil
}
