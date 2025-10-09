package protocols

import "github.com/charlesng35/shellcn/internal/drivers"

// Protocol models catalog metadata used to describe a remote access protocol.
type Protocol struct {
	ID           string
	DriverID     string
	Module       string
	Title        string
	Description  string
	Category     string
	Icon         string
	DefaultPort  int
	SortOrder    int
	Features     []string
	Capabilities drivers.Capabilities
}

func cloneProtocol(proto *Protocol) *Protocol {
	if proto == nil {
		return nil
	}
	cp := *proto
	if len(proto.Features) > 0 {
		cp.Features = append([]string(nil), proto.Features...)
	}
	if proto.Capabilities.Extras != nil {
		extras := make(map[string]bool, len(proto.Capabilities.Extras))
		for k, v := range proto.Capabilities.Extras {
			extras[k] = v
		}
		cp.Capabilities.Extras = extras
	}
	return &cp
}
