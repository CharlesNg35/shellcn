package grpcplugin

import (
	"slices"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestWireStorageListFilterRoundTrip(t *testing.T) {
	in := plugin.StorageListFilter{
		Keys:          []string{"prod/restart", "prod/status"},
		KeyPrefix:     "prod/",
		ContentType:   "application/json",
		CreatedAfter:  time.Unix(10, 1),
		CreatedBefore: time.Unix(20, 2),
		UpdatedAfter:  time.Unix(30, 3),
		UpdatedBefore: time.Unix(40, 4),
		Limit:         25,
		Offset:        50,
	}

	wire := wireStorageListFilter(&in)
	got := pluginStorageListFilter(wire)
	if got == nil {
		t.Fatal("filter round trip returned nil")
	}
	if !slices.Equal(got.Keys, in.Keys) ||
		got.KeyPrefix != in.KeyPrefix ||
		got.ContentType != in.ContentType ||
		!got.CreatedAfter.Equal(in.CreatedAfter) ||
		!got.CreatedBefore.Equal(in.CreatedBefore) ||
		!got.UpdatedAfter.Equal(in.UpdatedAfter) ||
		!got.UpdatedBefore.Equal(in.UpdatedBefore) ||
		got.Limit != in.Limit ||
		got.Offset != in.Offset {
		t.Fatalf("filter round trip mismatch: got %+v want %+v", *got, in)
	}
}

func TestPluginStorageListFilterNil(t *testing.T) {
	got := pluginStorageListFilter((*pluginv1.StorageListFilter)(nil))
	if got != nil {
		t.Fatalf("nil filter should map to nil, got %+v", got)
	}
	if wire := wireStorageListFilter(nil); wire != nil {
		t.Fatalf("nil SDK filter should map to nil wire filter, got %+v", wire)
	}
}
