package cluster

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

type StoreOwnerRegistry struct {
	store store.ClusterOwnerStore
	now   func() time.Time
}

func NewStoreOwnerRegistry(store store.ClusterOwnerStore) *StoreOwnerRegistry {
	return &StoreOwnerRegistry{store: store, now: time.Now}
}

func (r *StoreOwnerRegistry) Claim(ctx context.Context, key string, instance InstanceRef, opts ClaimOptions) (Lease, error) {
	if r.store == nil {
		return nil, fmt.Errorf("cluster: owner store is required")
	}
	if key == "" {
		return nil, fmt.Errorf("cluster: owner key is required")
	}
	if instance.ID == "" {
		return nil, fmt.Errorf("cluster: instance ID is required")
	}
	opts = opts.withDefaults()
	now := r.now().UTC()
	owner := &models.ClusterOwner{
		Key:          key,
		InstanceID:   instance.ID,
		InternalURL:  instance.PreferredInternalURL(),
		InternalURLs: encodeInternalURLCandidates(instance.InternalURLCandidates()),
		LeaseID:      randomLeaseID(),
		ExpiresAt:    now.Add(opts.TTL),
	}
	claimed, err := r.store.Claim(ctx, owner, opts.Mode == ClaimReplace, now)
	if errors.Is(err, models.ErrConflict) {
		return nil, fmt.Errorf("%w: %s", ErrOwnedElsewhere, claimed.InstanceID)
	}
	if err != nil {
		return nil, err
	}
	ref := ownerRefFromModel(claimed)
	return &storeLease{registry: r, owner: ref, ttl: opts.TTL}, nil
}

func (r *StoreOwnerRegistry) Get(ctx context.Context, key string) (OwnerRef, bool, error) {
	owner, err := r.store.Get(ctx, key, r.now().UTC())
	if errors.Is(err, store.ErrNotFound) {
		return OwnerRef{}, false, nil
	}
	if err != nil {
		return OwnerRef{}, false, err
	}
	return ownerRefFromModel(owner), true, nil
}

func (r *StoreOwnerRegistry) PreferInternalURL(ctx context.Context, owner OwnerRef, internalURL string) error {
	if r.store == nil {
		return fmt.Errorf("cluster: owner store is required")
	}
	if owner.Key == "" || owner.LeaseID == "" || internalURL == "" {
		return nil
	}
	_, err := r.store.PreferInternalURL(ctx, owner.Key, owner.LeaseID, internalURL, r.now().UTC())
	return err
}

type storeLease struct {
	registry *StoreOwnerRegistry
	owner    OwnerRef
	ttl      time.Duration
}

func (l *storeLease) Owner() OwnerRef {
	return l.owner
}

func (l *storeLease) Renew(ctx context.Context) error {
	now := l.registry.now().UTC()
	expiresAt := now.Add(l.ttl)
	ok, err := l.registry.store.Renew(ctx, l.owner.Key, l.owner.LeaseID, expiresAt, now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLeaseExpired
	}
	l.owner.ExpiresAt = expiresAt
	return nil
}

func (l *storeLease) Release(ctx context.Context) error {
	return l.registry.store.Release(ctx, l.owner.Key, l.owner.LeaseID)
}

func ownerRefFromModel(owner models.ClusterOwner) OwnerRef {
	return OwnerRef{
		Instance:  newInstanceRefWithPreferred(owner.InstanceID, owner.InternalURL, decodeInternalURLCandidates(owner.InternalURLs)),
		Key:       owner.Key,
		LeaseID:   owner.LeaseID,
		ExpiresAt: owner.ExpiresAt,
	}
}

func randomLeaseID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("cluster: crypto/rand failed: " + err.Error())
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
