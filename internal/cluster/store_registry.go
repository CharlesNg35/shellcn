package cluster

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
		Key:         key,
		InstanceID:  instance.ID,
		InternalURL: instance.InternalURL,
		LeaseID:     randomLeaseID(),
		ExpiresAt:   now.Add(opts.TTL),
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
		Instance: InstanceRef{
			ID:          owner.InstanceID,
			InternalURL: owner.InternalURL,
		},
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
