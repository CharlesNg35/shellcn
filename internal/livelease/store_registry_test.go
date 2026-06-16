package livelease

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/store"
)

func TestStoreLeaseRegistryExclusiveAndReplace(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreLeaseRegistry(st.LiveStateLeases)
	a := NewInstanceRef("a", "http://a")
	b := NewInstanceRef("b", "http://b")

	leaseA, err := reg.Claim(context.Background(), "session:c:u", a, ClaimOptions{Mode: ClaimExclusive, TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim a: %v", err)
	}
	if _, err := reg.Claim(context.Background(), "session:c:u", b, ClaimOptions{Mode: ClaimExclusive, TTL: time.Minute}); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("claim b: want ErrLeaseHeld, got %v", err)
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
	ref, ok, err := reg.Get(context.Background(), "agent:c")
	if err != nil || !ok {
		t.Fatalf("agent lease lookup: ok=%v err=%v", ok, err)
	}
	if ref.Instance.ID != "b" || ref.LeaseID != leaseB.Ref().LeaseID {
		t.Fatalf("agent lease = %+v, lease = %+v", ref, leaseB.Ref())
	}
}

func TestStoreLeaseRegistryRenewAndRelease(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreLeaseRegistry(st.LiveStateLeases)

	lease, err := reg.Claim(context.Background(), "session:c:u", NewInstanceRef("a", "http://a"), ClaimOptions{TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	before := lease.Ref().ExpiresAt
	if err := lease.Renew(context.Background()); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !lease.Ref().ExpiresAt.After(before) {
		t.Fatalf("renew did not extend expiry: before=%s after=%s", before, lease.Ref().ExpiresAt)
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, ok, err := reg.Get(context.Background(), "session:c:u"); err != nil || ok {
		t.Fatalf("released lease should be gone: ok=%v err=%v", ok, err)
	}
}

func TestStoreLeaseRegistryStoresAndPromotesInternalURLs(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreLeaseRegistry(st.LiveStateLeases)
	lease, err := reg.Claim(context.Background(), "session:c:u", NewInstanceRef("a", "http://a1", "http://a2"), ClaimOptions{TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	ref, ok, err := reg.Get(context.Background(), "session:c:u")
	if err != nil || !ok {
		t.Fatalf("get lease: ok=%v err=%v", ok, err)
	}
	if got := ref.InternalURLCandidates(); len(got) != 2 || got[0] != "http://a1" || got[1] != "http://a2" {
		t.Fatalf("candidates = %#v", got)
	}
	if err := reg.PreferInternalURL(context.Background(), ref, "http://a2"); err != nil {
		t.Fatalf("prefer url: %v", err)
	}
	ref, ok, err = reg.Get(context.Background(), "session:c:u")
	if err != nil || !ok {
		t.Fatalf("get lease after prefer: ok=%v err=%v", ok, err)
	}
	if ref.Instance.PreferredInternalURL() != "http://a2" {
		t.Fatalf("preferred URL = %q", ref.Instance.PreferredInternalURL())
	}

	stale := ref
	stale.LeaseID = "old-lease"
	if err := reg.PreferInternalURL(context.Background(), stale, "http://stale"); err != nil {
		t.Fatalf("stale prefer url: %v", err)
	}
	ref, ok, err = reg.Get(context.Background(), "session:c:u")
	if err != nil || !ok {
		t.Fatalf("get lease after stale prefer: ok=%v err=%v", ok, err)
	}
	if ref.Instance.PreferredInternalURL() != "http://a2" {
		t.Fatalf("stale prefer changed URL to %q", ref.Instance.PreferredInternalURL())
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestStoreLeaseRegistryReplaceClaimPreservesReachablePreferredInternalURL(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreLeaseRegistry(st.LiveStateLeases)
	ctx := context.Background()

	first, err := reg.Claim(ctx, "agent:c", NewInstanceRef("a", "http://old1", "http://old2"), ClaimOptions{Mode: ClaimReplace, TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim first lease: %v", err)
	}
	ref, ok, err := reg.Get(ctx, "agent:c")
	if err != nil || !ok {
		t.Fatalf("get first lease: ok=%v err=%v", ok, err)
	}
	if err := reg.PreferInternalURL(ctx, ref, "http://old2"); err != nil {
		t.Fatalf("prefer old url: %v", err)
	}

	second, err := reg.Claim(ctx, "agent:c", NewInstanceRef("a", "http://new1", "http://old2"), ClaimOptions{Mode: ClaimReplace, TTL: time.Minute})
	if err != nil {
		t.Fatalf("claim replacement lease: %v", err)
	}
	ref, ok, err = reg.Get(ctx, "agent:c")
	if err != nil || !ok {
		t.Fatalf("get replacement lease: ok=%v err=%v", ok, err)
	}
	if ref.LeaseID != second.Ref().LeaseID {
		t.Fatalf("lease id = %q, want %q", ref.LeaseID, second.Ref().LeaseID)
	}
	if ref.Instance.PreferredInternalURL() != "http://old2" {
		t.Fatalf("preferred URL after replacement = %q", ref.Instance.PreferredInternalURL())
	}
	if got := ref.InternalURLCandidates(); len(got) != 2 || got[0] != "http://old2" || got[1] != "http://new1" {
		t.Fatalf("replacement candidates = %#v", got)
	}
	if err := reg.PreferInternalURL(ctx, first.Ref(), "http://new1"); err != nil {
		t.Fatalf("stale prefer after replacement: %v", err)
	}
	ref, ok, err = reg.Get(ctx, "agent:c")
	if err != nil || !ok {
		t.Fatalf("get after stale prefer: ok=%v err=%v", ok, err)
	}
	if ref.Instance.PreferredInternalURL() != "http://old2" {
		t.Fatalf("stale lease changed preferred URL to %q", ref.Instance.PreferredInternalURL())
	}
}

func TestStoreLeaseRegistryReplaceClaimResetsMissingPreferredInternalURL(t *testing.T) {
	st := store.NewMemory()
	reg := NewStoreLeaseRegistry(st.LiveStateLeases)
	ctx := context.Background()

	if _, err := reg.Claim(ctx, "agent:c", NewInstanceRef("a", "http://old1", "http://old2"), ClaimOptions{Mode: ClaimReplace, TTL: time.Minute}); err != nil {
		t.Fatalf("claim first lease: %v", err)
	}
	ref, ok, err := reg.Get(ctx, "agent:c")
	if err != nil || !ok {
		t.Fatalf("get first lease: ok=%v err=%v", ok, err)
	}
	if err := reg.PreferInternalURL(ctx, ref, "http://old2"); err != nil {
		t.Fatalf("prefer old url: %v", err)
	}

	if _, err := reg.Claim(ctx, "agent:c", NewInstanceRef("a", "http://new1", "http://new2"), ClaimOptions{Mode: ClaimReplace, TTL: time.Minute}); err != nil {
		t.Fatalf("claim replacement lease: %v", err)
	}
	ref, ok, err = reg.Get(ctx, "agent:c")
	if err != nil || !ok {
		t.Fatalf("get replacement lease: ok=%v err=%v", ok, err)
	}
	if ref.Instance.PreferredInternalURL() != "http://new1" {
		t.Fatalf("preferred URL after missing old URL = %q", ref.Instance.PreferredInternalURL())
	}
	if got := ref.InternalURLCandidates(); len(got) != 2 || got[0] != "http://new1" || got[1] != "http://new2" {
		t.Fatalf("replacement candidates = %#v", got)
	}
}
