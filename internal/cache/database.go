package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/models"
)

// DatabaseStore implements the cache Store interface using the primary SQL database.
type DatabaseStore struct {
	db *gorm.DB
}

// NewDatabaseStore constructs a database-backed Store.
func NewDatabaseStore(db *gorm.DB) *DatabaseStore {
	if db == nil {
		return nil
	}
	return &DatabaseStore{db: db}
}

// IncrementWithTTL atomically increments a counter for the supplied key.
func (s *DatabaseStore) IncrementWithTTL(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	if s == nil {
		return 0, 0, errors.New("cache: database store not initialised")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if window <= 0 {
		window = time.Minute
	}

	now := time.Now()
	expiry := now.Add(window)

	var (
		count int64
		err   error
	)

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entry models.CacheEntry
		// Acquire row-level lock
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Take(&entry, "key = ?", key).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			count = 1
			entry = models.CacheEntry{
				Key:       key,
				Value:     []byte(strconv.FormatInt(count, 10)),
				ExpiresAt: expiry,
			}
			return tx.Create(&entry).Error
		}
		if err != nil {
			return err
		}

		if entry.ExpiresAt.Before(now) {
			count = 1
			entry.Value = []byte("1")
			entry.ExpiresAt = expiry
		} else {
			current, _ := strconv.ParseInt(string(entry.Value), 10, 64)
			count = current + 1
			entry.Value = []byte(strconv.FormatInt(count, 10))
			entry.ExpiresAt = expiry
		}

		return tx.Save(&entry).Error
	})
	if err != nil {
		return 0, 0, err
	}

	return count, expiry.Sub(now), nil
}

// Set upserts the value for a given key with expiry.
func (s *DatabaseStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s == nil {
		return errors.New("cache: database store not initialised")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	expiry := time.Time{}
	if ttl > 0 {
		expiry = time.Now().Add(ttl)
	}

	entry := models.CacheEntry{
		Key:       key,
		Value:     value,
		ExpiresAt: expiry,
	}

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "expires_at", "updated_at"}),
		}).Create(&entry).Error
}

// Get retrieves a value by key, respecting expiry.
func (s *DatabaseStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if s == nil {
		return nil, false, errors.New("cache: database store not initialised")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var entry models.CacheEntry
	err := s.db.WithContext(ctx).Take(&entry, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		_ = s.Delete(ctx, key)
		return nil, false, nil
	}

	return entry.Value, true, nil
}

// Delete removes keys from the store.
func (s *DatabaseStore) Delete(ctx context.Context, keys ...string) error {
	if s == nil {
		return errors.New("cache: database store not initialised")
	}
	if len(keys) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return s.db.WithContext(ctx).Where("key IN ?", keys).Delete(&models.CacheEntry{}).Error
}
