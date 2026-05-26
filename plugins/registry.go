package plugins

import (
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/docker"
	"github.com/charlesng/shellcn/plugins/proxmox"
	"github.com/charlesng/shellcn/plugins/rdp"
	"github.com/charlesng/shellcn/plugins/sftp"
	"github.com/charlesng/shellcn/plugins/ssh"
	"github.com/charlesng/shellcn/plugins/vnc"
)

// Register wires every first-party plugin into the registry. This is the single
// place to add a new protocol plugin — append its constructor to all().
func Register(reg *plugin.Registry) {
	for _, p := range all() {
		reg.MustRegister(p)
	}
}

// all returns the first-party plugin set in registration order.
func all() []plugin.Plugin {
	return []plugin.Plugin{
		ssh.New(),
		sftp.New(),
		docker.New(),
		vnc.New(),
		rdp.New(),
		proxmox.New(),
	}
}
