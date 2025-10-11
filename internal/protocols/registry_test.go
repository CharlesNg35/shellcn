package protocols

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/drivers"
)

type fakeDriver struct {
	desc drivers.Descriptor
	cap  drivers.Capabilities
	err  error
}

func (f *fakeDriver) Descriptor() drivers.Descriptor { return f.desc }
func (f *fakeDriver) Capabilities(ctx context.Context) (drivers.Capabilities, error) {
	if f.cap.Extras == nil {
		f.cap.Extras = map[string]bool{}
	}
	return f.cap, f.err
}

func TestRegisterFromDriver(t *testing.T) {
	driverReg := drivers.NewRegistry()
	driverReg.MustRegister(&fakeDriver{
		desc: drivers.Descriptor{ID: "ssh", Title: "SSH", Category: "terminal", SortOrder: 1},
		cap:  drivers.Capabilities{Terminal: true, FileTransfer: true},
	})

	protoReg := NewRegistry()
	if err := protoReg.SyncFromDrivers(context.Background(), driverReg); err != nil {
		t.Fatalf("sync returned error: %v", err)
	}

	proto, ok := protoReg.Get("ssh")
	if !ok {
		t.Fatalf("expected protocol to be registered")
	}
	if proto.DriverID != "ssh" {
		t.Fatalf("expected driver id ssh, got %s", proto.DriverID)
	}
	if len(proto.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(proto.Features))
	}
}

func TestRegisterValidation(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(nil); !errors.Is(err, errNilProtocol) {
		t.Fatalf("expected errNilProtocol, got %v", err)
	}
	if err := reg.Register(&Protocol{}); !errors.Is(err, errEmptyProtocol) {
		t.Fatalf("expected errEmptyProtocol, got %v", err)
	}
	if err := reg.Register(&Protocol{ID: "ssh"}); !errors.Is(err, errEmptyDriver) {
		t.Fatalf("expected errEmptyDriver, got %v", err)
	}
}

func TestDuplicateRegistration(t *testing.T) {
	reg := NewRegistry()
	proto := &Protocol{ID: "ssh", DriverID: "ssh"}
	if err := reg.Register(proto); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := reg.Register(proto); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestGetAllSorting(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(&Protocol{ID: "docker", DriverID: "docker", SortOrder: 5})
	reg.MustRegister(&Protocol{ID: "ssh", DriverID: "ssh", SortOrder: 1})
	reg.MustRegister(&Protocol{ID: "database", DriverID: "database", SortOrder: 5})

	list := reg.GetAll()
	expected := []string{"ssh", "database", "docker"}
	if len(list) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(list))
	}
	for i, id := range expected {
		if list[i].ID != id {
			t.Fatalf("expected %s at index %d, got %s", id, i, list[i].ID)
		}
	}
}

func TestSyncFromDriversPropagatesErrors(t *testing.T) {
	driverReg := drivers.NewRegistry()
	driverReg.MustRegister(&fakeDriver{desc: drivers.Descriptor{ID: "ssh"}, err: errors.New("caps fail")})

	protoReg := NewRegistry()
	if err := protoReg.SyncFromDrivers(context.Background(), driverReg); err == nil {
		t.Fatalf("expected sync error for capabilities failure")
	}
}
