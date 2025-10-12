package drivers

import (
	"context"
	"errors"
	"testing"
)

type stubDriver struct {
	BaseDriver
	cap Capabilities
	err error
}

func newStubDriver(desc Descriptor, cap Capabilities, err error) *stubDriver {
	return &stubDriver{
		BaseDriver: NewBaseDriver(desc),
		cap:        cap,
		err:        err,
	}
}

func (s *stubDriver) Capabilities(ctx context.Context) (Capabilities, error) {
	return s.cap, s.err
}

func TestRegisterAndFetchDriver(t *testing.T) {
	repo := NewRegistry()
	drv := newStubDriver(Descriptor{ID: "ssh", Title: "SSH"}, Capabilities{}, nil)
	if err := repo.Register(drv); err != nil {
		t.Fatalf("expected register success, got %v", err)
	}

	stored, ok := repo.Get("ssh")
	if !ok || stored != drv {
		t.Fatalf("expected driver to be retrievable")
	}
}

func TestRegisterValidatesID(t *testing.T) {
	repo := NewRegistry()
	if err := repo.Register(nil); !errors.Is(err, ErrNilDriver) {
		t.Fatalf("expected ErrNilDriver, got %v", err)
	}

	drv := newStubDriver(Descriptor{ID: ""}, Capabilities{}, nil)
	if err := repo.Register(drv); !errors.Is(err, ErrEmptyDriverID) {
		t.Fatalf("expected ErrEmptyDriverID, got %v", err)
	}
}

func TestRegisterDuplicateID(t *testing.T) {
	repo := NewRegistry()
	drv := newStubDriver(Descriptor{ID: "ssh"}, Capabilities{}, nil)
	if err := repo.Register(drv); err != nil {
		t.Fatalf("expected register success, got %v", err)
	}

	dup := newStubDriver(Descriptor{ID: "ssh"}, Capabilities{}, nil)
	if err := repo.Register(dup); !errors.Is(err, ErrDuplicateDriverID) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestDescribeSortsByOrderAndID(t *testing.T) {
	repo := NewRegistry()
	repo.MustRegister(newStubDriver(Descriptor{ID: "kubernetes", SortOrder: 10}, Capabilities{}, nil))
	repo.MustRegister(newStubDriver(Descriptor{ID: "ssh", SortOrder: 1}, Capabilities{}, nil))
	repo.MustRegister(newStubDriver(Descriptor{ID: "docker", SortOrder: 10}, Capabilities{}, nil))
	repo.MustRegister(newStubDriver(Descriptor{ID: "database", SortOrder: 5}, Capabilities{}, nil))

	descriptors, err := repo.Describe(context.Background())
	if err != nil {
		t.Fatalf("describe returned error: %v", err)
	}

	expected := []string{"ssh", "database", "docker", "kubernetes"}
	if len(descriptors) != len(expected) {
		t.Fatalf("expected %d descriptors, got %d", len(expected), len(descriptors))
	}
	for i, id := range expected {
		if descriptors[i].ID != id {
			t.Fatalf("expected id %s at index %d, got %s", id, i, descriptors[i].ID)
		}
	}
}

func TestCapabilitiesFetch(t *testing.T) {
	repo := NewRegistry()
	repo.MustRegister(newStubDriver(
		Descriptor{ID: "ssh"},
		Capabilities{Terminal: true},
		nil,
	))

	caps, err := repo.Capabilities(context.Background(), "ssh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !caps.Terminal {
		t.Fatalf("expected terminal capability to be true")
	}
	if caps.Extras == nil {
		t.Fatalf("expected extras map to be initialised")
	}
}

func TestCapabilitiesMissingDriver(t *testing.T) {
	repo := NewRegistry()
	if _, err := repo.Capabilities(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error for missing driver")
	}
}

func TestCapabilitiesErrorPropagates(t *testing.T) {
	repo := NewRegistry()
	repo.MustRegister(newStubDriver(
		Descriptor{ID: "ssh"},
		Capabilities{},
		errors.New("cap failure"),
	))

	if _, err := repo.Capabilities(context.Background(), "ssh"); err == nil || err.Error() != "cap failure" {
		t.Fatalf("expected wrapped capability error, got %v", err)
	}
}
