package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/store"
)

func TestStoreOwnerRegistryExclusiveAndReplace(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreOwnerRegistry(st.ClusterOwners)
	a := NewInstanceRef("a", "http://a")
	b := NewInstanceRef("b", "http://b")

	leaseA, err := reg.Claim(context.Background(), "session:c:u", a, ClaimOptions{Mode: ClaimExclusive, TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim a: %v", err)
	}
	if _, err := reg.Claim(context.Background(), "session:c:u", b, ClaimOptions{Mode: ClaimExclusive, TTL: time.Minute}); !errors.Is(err, ErrOwnedElsewhere) {
		t.Fatalf("claim b: want ErrOwnedElsewhere, got %v", err)
	}
	if err := leaseA.Release(context.Background()); err != nil {
		t.Fatalf("release a: %v", err)
	}

	if _, err := reg.Claim(context.Background(), "agent:c", a, ClaimOptions{Mode: ClaimReplace, TTL: time.Minute}); err != nil {
		t.Fatalf("agent claim a: %v", err)
	}
	leaseB, err := reg.Claim(context.Background(), "agent:c", b, ClaimOptions{Mode: ClaimReplace, TTL: time.Minute})
	if err != nil {
		t.Fatalf("agent claim b: %v", err)
	}
	owner, ok, err := reg.Get(context.Background(), "agent:c")
	if err != nil || !ok {
		t.Fatalf("agent owner lookup: ok=%v err=%v", ok, err)
	}
	if owner.Instance.ID != "b" || owner.LeaseID != leaseB.Owner().LeaseID {
		t.Fatalf("agent owner = %+v, lease = %+v", owner, leaseB.Owner())
	}
}

func TestStoreOwnerRegistryRenewAndRelease(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreOwnerRegistry(st.ClusterOwners)

	lease, err := reg.Claim(context.Background(), "session:c:u", NewInstanceRef("a", "http://a"), ClaimOptions{TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	before := lease.Owner().ExpiresAt
	if err := lease.Renew(context.Background()); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !lease.Owner().ExpiresAt.After(before) {
		t.Fatalf("renew did not extend expiry: before=%s after=%s", before, lease.Owner().ExpiresAt)
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, ok, err := reg.Get(context.Background(), "session:c:u"); err != nil || ok {
		t.Fatalf("released owner should be gone: ok=%v err=%v", ok, err)
	}
}

func TestStoreOwnerRegistryStoresAndPromotesInternalURLs(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreOwnerRegistry(st.ClusterOwners)
	lease, err := reg.Claim(context.Background(), "session:c:u", NewInstanceRef("a", "http://a1", "http://a2"), ClaimOptions{TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	owner, ok, err := reg.Get(context.Background(), "session:c:u")
	if err != nil || !ok {
		t.Fatalf("get owner: ok=%v err=%v", ok, err)
	}
	if got := owner.InternalURLCandidates(); len(got) != 2 || got[0] != "http://a1" || got[1] != "http://a2" {
		t.Fatalf("candidates = %#v", got)
	}
	if err := reg.PreferInternalURL(context.Background(), owner, "http://a2"); err != nil {
		t.Fatalf("prefer url: %v", err)
	}
	owner, ok, err = reg.Get(context.Background(), "session:c:u")
	if err != nil || !ok {
		t.Fatalf("get owner after prefer: ok=%v err=%v", ok, err)
	}
	if owner.Instance.PreferredInternalURL() != "http://a2" {
		t.Fatalf("preferred URL = %q", owner.Instance.PreferredInternalURL())
	}

	stale := owner
	stale.LeaseID = "old-lease"
	if err := reg.PreferInternalURL(context.Background(), stale, "http://stale"); err != nil {
		t.Fatalf("stale prefer url: %v", err)
	}
	owner, ok, err = reg.Get(context.Background(), "session:c:u")
	if err != nil || !ok {
		t.Fatalf("get owner after stale prefer: ok=%v err=%v", ok, err)
	}
	if owner.Instance.PreferredInternalURL() != "http://a2" {
		t.Fatalf("stale prefer changed URL to %q", owner.Instance.PreferredInternalURL())
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("release: %v", err)
	}
}
