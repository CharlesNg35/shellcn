package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/internal/cache"
	"github.com/charlesng35/shellcn/internal/models"
)

const sessionCacheKeyPrefix = "auth:sessions:refresh:"

// NewRedisSessionCache wraps the shared Redis client inside a SessionCache implementation.
func NewRedisSessionCache(client cache.Store) SessionCache {
	return newSessionStoreCache(client)
}

// NewDatabaseSessionCache provides a session cache backed by the relational database.
func NewDatabaseSessionCache(store cache.Store) SessionCache {
	return newSessionStoreCache(store)
}

type sessionStoreCache struct {
	store cache.Store
}

func newSessionStoreCache(store cache.Store) SessionCache {
	if store == nil {
		return nil
	}
	return &sessionStoreCache{store: store}
}

func (c *sessionStoreCache) Get(ctx context.Context, refreshToken string) (*models.Session, error) {
	key := cacheKey(refreshToken)
	if key == "" {
		return nil, errSessionCacheMiss
	}

	data, found, err := c.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errSessionCacheMiss
	}

	var session models.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("session cache: decode: %w", err)
	}
	return &session, nil
}

func (c *sessionStoreCache) Set(ctx context.Context, session *models.Session, ttl time.Duration) error {
	if session == nil {
		return errors.New("session cache: session is nil")
	}
	key := cacheKey(session.RefreshToken)
	if key == "" {
		return errors.New("session cache: refresh token missing")
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("session cache: marshal: %w", err)
	}
	if ttl <= 0 {
		ttl = time.Second
	}

	return c.store.Set(ctx, key, payload, ttl)
}

func (c *sessionStoreCache) Delete(ctx context.Context, refreshToken string) error {
	key := cacheKey(refreshToken)
	if key == "" {
		return nil
	}
	return c.store.Delete(ctx, key)
}

func cacheKey(refreshToken string) string {
	token := strings.TrimSpace(refreshToken)
	if token == "" {
		return ""
	}
	return sessionCacheKeyPrefix + token
}
