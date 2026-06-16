package livelease

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

type StoreLeaseRegistry struct {
	store store.LiveStateLeaseStore
	now   func() time.Time
}

func NewStoreLeaseRegistry(store store.LiveStateLeaseStore) *StoreLeaseRegistry {
	return &StoreLeaseRegistry{store: store, now: time.Now}
}

func (r *StoreLeaseRegistry) Claim(ctx context.Context, key string, instance InstanceRef, opts ClaimOptions) (Lease, error) {
	if r.store == nil {
		return nil, fmt.Errorf("livelease: lease store is required")
	}
	if key == "" {
		return nil, fmt.Errorf("livelease: lease key is required")
	}
	if instance.ID == "" {
		return nil, fmt.Errorf("livelease: instance ID is required")
	}
	opts = opts.withDefaults()
	now := r.now().UTC()
	lease := &models.LiveStateLease{
		Key:          key,
		InstanceID:   instance.ID,
		InternalURL:  instance.PreferredInternalURL(),
		InternalURLs: encodeInternalURLCandidates(instance.InternalURLCandidates()),
		LeaseID:      randomLeaseID(),
		ExpiresAt:    now.Add(opts.TTL),
	}
	claimed, err := r.store.Claim(ctx, lease, opts.Mode == ClaimReplace, now)
	if errors.Is(err, models.ErrConflict) {
		return nil, fmt.Errorf("%w: %s", ErrLeaseHeld, claimed.InstanceID)
	}
	if err != nil {
		return nil, err
	}
	ref := leaseRefFromModel(claimed)
	return &storeLease{registry: r, ref: ref, ttl: opts.TTL}, nil
}

func (r *StoreLeaseRegistry) Get(ctx context.Context, key string) (LeaseRef, bool, error) {
	lease, err := r.store.Get(ctx, key, r.now().UTC())
	if errors.Is(err, store.ErrNotFound) {
		return LeaseRef{}, false, nil
	}
	if err != nil {
		return LeaseRef{}, false, err
	}
	return leaseRefFromModel(lease), true, nil
}

func (r *StoreLeaseRegistry) PreferInternalURL(ctx context.Context, ref LeaseRef, internalURL string) error {
	if r.store == nil {
		return fmt.Errorf("livelease: lease store is required")
	}
	if ref.Key == "" || ref.LeaseID == "" || internalURL == "" {
		return nil
	}
	_, err := r.store.PreferInternalURL(ctx, ref.Key, ref.LeaseID, internalURL, r.now().UTC())
	return err
}

type storeLease struct {
	registry *StoreLeaseRegistry
	ref      LeaseRef
	ttl      time.Duration
}

func (l *storeLease) Ref() LeaseRef {
	return l.ref
}

func (l *storeLease) Renew(ctx context.Context) error {
	now := l.registry.now().UTC()
	expiresAt := now.Add(l.ttl)
	ok, err := l.registry.store.Renew(ctx, l.ref.Key, l.ref.LeaseID, expiresAt, now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLeaseExpired
	}
	l.ref.ExpiresAt = expiresAt
	return nil
}

func (l *storeLease) Release(ctx context.Context) error {
	return l.registry.store.Release(ctx, l.ref.Key, l.ref.LeaseID)
}

func leaseRefFromModel(lease models.LiveStateLease) LeaseRef {
	return LeaseRef{
		Instance:  newInstanceRefWithPreferred(lease.InstanceID, lease.InternalURL, decodeInternalURLCandidates(lease.InternalURLs)),
		Key:       lease.Key,
		LeaseID:   lease.LeaseID,
		ExpiresAt: lease.ExpiresAt,
	}
}

func randomLeaseID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("livelease: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func encodeInternalURLCandidates(urls []string) string {
	urls = uniqueNonEmpty(urls)
	if len(urls) == 0 {
		return ""
	}
	b, err := json.Marshal(urls)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeInternalURLCandidates(raw string) []string {
	var urls []string
	if err := json.Unmarshal([]byte(raw), &urls); err != nil {
		return nil
	}
	return uniqueNonEmpty(urls)
}
