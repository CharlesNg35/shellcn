package plugins

import (
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/cassandra"
	"github.com/charlesng/shellcn/plugins/clickhouse"
	"github.com/charlesng/shellcn/plugins/cockroachdb"
	"github.com/charlesng/shellcn/plugins/docker"
	"github.com/charlesng/shellcn/plugins/ftp"
	"github.com/charlesng/shellcn/plugins/ftps"
	"github.com/charlesng/shellcn/plugins/kafka"
	"github.com/charlesng/shellcn/plugins/minio"
	"github.com/charlesng/shellcn/plugins/mongodb"
	"github.com/charlesng/shellcn/plugins/mssql"
	"github.com/charlesng/shellcn/plugins/mysql"
	"github.com/charlesng/shellcn/plugins/nats"
	"github.com/charlesng/shellcn/plugins/nfs"
	"github.com/charlesng/shellcn/plugins/oracle"
	"github.com/charlesng/shellcn/plugins/postgresql"
	"github.com/charlesng/shellcn/plugins/proxmox"
	"github.com/charlesng/shellcn/plugins/rabbitmq"
	"github.com/charlesng/shellcn/plugins/rdp"
	"github.com/charlesng/shellcn/plugins/redis"
	"github.com/charlesng/shellcn/plugins/s3"
	"github.com/charlesng/shellcn/plugins/sftp"
	"github.com/charlesng/shellcn/plugins/smb"
	"github.com/charlesng/shellcn/plugins/ssh"
	"github.com/charlesng/shellcn/plugins/telnet"
	"github.com/charlesng/shellcn/plugins/vnc"
	"github.com/charlesng/shellcn/plugins/webdav"
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
		telnet.New(),
		ftp.New(),
		ftps.New(),
		webdav.New(),
		smb.New(),
		nfs.New(),
		s3.New(),
		minio.New(),
		docker.New(),
		vnc.New(),
		rdp.New(),
		proxmox.New(),
		postgresql.New(),
		mysql.New(),
		redis.New(),
		mongodb.New(),
		mssql.New(),
		oracle.New(),
		cockroachdb.New(),
		clickhouse.New(),
		cassandra.New(),
		rabbitmq.New(),
		kafka.New(),
		nats.New(),
	}
}
