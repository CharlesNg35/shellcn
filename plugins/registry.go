package plugins

import (
	"github.com/charlesng35/shellcn/plugins/docker"
	"github.com/charlesng35/shellcn/plugins/ftp"
	"github.com/charlesng35/shellcn/plugins/ftps"
	"github.com/charlesng35/shellcn/plugins/kubernetes"
	"github.com/charlesng35/shellcn/plugins/ldap"
	"github.com/charlesng35/shellcn/plugins/mongodb"
	"github.com/charlesng35/shellcn/plugins/mysql"
	"github.com/charlesng35/shellcn/plugins/podman"
	"github.com/charlesng35/shellcn/plugins/postgresql"
	"github.com/charlesng35/shellcn/plugins/proxmox"
	"github.com/charlesng35/shellcn/plugins/rdp"
	"github.com/charlesng35/shellcn/plugins/redis"
	"github.com/charlesng35/shellcn/plugins/s3"
	"github.com/charlesng35/shellcn/plugins/servermonitor"
	"github.com/charlesng35/shellcn/plugins/sftp"
	"github.com/charlesng35/shellcn/plugins/smb"
	"github.com/charlesng35/shellcn/plugins/ssh"
	"github.com/charlesng35/shellcn/plugins/swarm"
	"github.com/charlesng35/shellcn/plugins/vnc"
	"github.com/charlesng35/shellcn/plugins/webdav"
	"github.com/charlesng35/shellcn/sdk/plugin"
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
		ftp.New(),
		ftps.New(),
		webdav.New(),
		smb.New(),
		s3.New(),
		docker.New(),
		swarm.New(),
		podman.New(),
		vnc.New(),
		rdp.New(),
		proxmox.New(),
		kubernetes.New(),
		servermonitor.New(),
		postgresql.New(),
		mysql.New(),
		redis.New(),
		mongodb.New(),
		ldap.New(),
	}
}
