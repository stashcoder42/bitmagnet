package apikey

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/bitmagnet-io/bitmagnet/internal/database/dao"
	"github.com/bitmagnet-io/bitmagnet/internal/model"
	"gorm.io/gorm"
)

const (
	// KeyName is the key used to store the API key in the database
	KeyName = "api_key"
	// KeyLength is the number of random bytes to generate for the API key
	KeyLength = 32
)

var (
	ErrInvalidKey = errors.New("invalid API key")
)

// KeyInfo contains metadata about the API key
type KeyInfo struct {
	Key       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Service manages API key operations
type Service interface {
	// GetKey returns the current API key, generating one if it doesn't exist
	GetKey(ctx context.Context) (*KeyInfo, error)
	// ValidateKey checks if the provided key matches the stored key
	ValidateKey(ctx context.Context, key string) (bool, error)
	// RotateKey generates a new API key and invalidates the old one
	RotateKey(ctx context.Context) (*KeyInfo, error)
}

type service struct {
	dao *dao.Query
	mu  sync.RWMutex
	// cached key info for fast validation
	cached *KeyInfo
}

// NewService creates a new API key service
func NewService(dao *dao.Query) Service {
	return &service{
		dao: dao,
	}
}

func (s *service) GetKey(ctx context.Context) (*KeyInfo, error) {
	s.mu.RLock()
	if s.cached != nil {
		defer s.mu.RUnlock()
		return s.cached, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.cached != nil {
		return s.cached, nil
	}

	// Try to get from database
	kv, err := s.dao.KeyValue.WithContext(ctx).Where(s.dao.KeyValue.Key.Eq(KeyName)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Generate new key
			return s.generateAndStore(ctx)
		}
		return nil, err
	}

	s.cached = &KeyInfo{
		Key:       kv.Value,
		CreatedAt: kv.CreatedAt,
		UpdatedAt: kv.UpdatedAt,
	}

	return s.cached, nil
}

func (s *service) ValidateKey(ctx context.Context, key string) (bool, error) {
	info, err := s.GetKey(ctx)
	if err != nil {
		return false, err
	}

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(info.Key), []byte(key)) == 1, nil
}

func (s *service) RotateKey(ctx context.Context) (*KeyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.generateAndStore(ctx)
}

// generateAndStore creates a new API key and stores it in the database
// Must be called with s.mu held
func (s *service) generateAndStore(ctx context.Context) (*KeyInfo, error) {
	key, err := generateKey()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	kv := &model.KeyValue{
		Key:       KeyName,
		Value:     key,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Use Save which does upsert (insert on conflict update)
	if err := s.dao.KeyValue.WithContext(ctx).Save(kv); err != nil {
		return nil, err
	}

	// Re-fetch to get the actual timestamps from the database
	stored, err := s.dao.KeyValue.WithContext(ctx).Where(s.dao.KeyValue.Key.Eq(KeyName)).First()
	if err != nil {
		return nil, err
	}

	s.cached = &KeyInfo{
		Key:       stored.Value,
		CreatedAt: stored.CreatedAt,
		UpdatedAt: stored.UpdatedAt,
	}

	return s.cached, nil
}

// generateKey creates a new random API key
func generateKey() (string, error) {
	bytes := make([]byte, KeyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
