package plugins

import (
	"github.com/charlesng35/shellcn/plugins/cassandra"
	"github.com/charlesng35/shellcn/plugins/clickhouse"
	"github.com/charlesng35/shellcn/plugins/cockroachdb"
	"github.com/charlesng35/shellcn/plugins/docker"
	"github.com/charlesng35/shellcn/plugins/dynamodb"
	"github.com/charlesng35/shellcn/plugins/elasticsearch"
	"github.com/charlesng35/shellcn/plugins/ftp"
	"github.com/charlesng35/shellcn/plugins/ftps"
	"github.com/charlesng35/shellcn/plugins/influxdb"
	"github.com/charlesng35/shellcn/plugins/kafka"
	"github.com/charlesng35/shellcn/plugins/kubernetes"
	"github.com/charlesng35/shellcn/plugins/ldap"
	"github.com/charlesng35/shellcn/plugins/meilisearch"
	"github.com/charlesng35/shellcn/plugins/minio"
	"github.com/charlesng35/shellcn/plugins/mongodb"
	"github.com/charlesng35/shellcn/plugins/mssql"
	"github.com/charlesng35/shellcn/plugins/mysql"
	"github.com/charlesng35/shellcn/plugins/nats"
	"github.com/charlesng35/shellcn/plugins/neo4j"
	"github.com/charlesng35/shellcn/plugins/nfs"
	"github.com/charlesng35/shellcn/plugins/opensearch"
	"github.com/charlesng35/shellcn/plugins/oracle"
	"github.com/charlesng35/shellcn/plugins/podman"
	"github.com/charlesng35/shellcn/plugins/postgresql"
	"github.com/charlesng35/shellcn/plugins/prometheus"
	"github.com/charlesng35/shellcn/plugins/proxmox"
	"github.com/charlesng35/shellcn/plugins/rabbitmq"
	"github.com/charlesng35/shellcn/plugins/rdp"
	"github.com/charlesng35/shellcn/plugins/redis"
	"github.com/charlesng35/shellcn/plugins/s3"
	"github.com/charlesng35/shellcn/plugins/servermonitor"
	"github.com/charlesng35/shellcn/plugins/sftp"
	"github.com/charlesng35/shellcn/plugins/smb"
	"github.com/charlesng35/shellcn/plugins/solr"
	"github.com/charlesng35/shellcn/plugins/ssh"
	"github.com/charlesng35/shellcn/plugins/swarm"
	"github.com/charlesng35/shellcn/plugins/telnet"
	"github.com/charlesng35/shellcn/plugins/typesense"
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
		telnet.New(),
		ftp.New(),
		ftps.New(),
		webdav.New(),
		smb.New(),
		nfs.New(),
		s3.New(),
		minio.New(),
		docker.New(),
		swarm.New(),
		podman.New(),
		vnc.New(),
		rdp.New(),
		proxmox.New(),
		kubernetes.New(),
		servermonitor.New(),
		prometheus.New(),
		influxdb.New(),
		postgresql.New(),
		mysql.New(),
		redis.New(),
		mongodb.New(),
		mssql.New(),
		oracle.New(),
		cockroachdb.New(),
		clickhouse.New(),
		cassandra.New(),
		dynamodb.New(),
		neo4j.New(),
		ldap.New(),
		elasticsearch.New(),
		opensearch.New(),
		meilisearch.New(),
		typesense.New(),
		solr.New(),
		rabbitmq.New(),
		kafka.New(),
		nats.New(),
	}
}
