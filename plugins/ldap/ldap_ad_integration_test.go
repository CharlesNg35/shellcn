package ldap

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/transport"
)

const (
	adRealm   = "CORP.EXAMPLE.COM"
	adBaseDN  = "dc=corp,dc=example,dc=com"
	adBindUPN = "Administrator@corp.example.com"
	adPwd     = "ShellcnAdmin123!"
)

// TestLDAPActiveDirectoryIntegration exercises the plugin against a real Active
// Directory implementation (Samba AD-DC). It is separate from the OpenLDAP test
// because an AD-DC needs a privileged container and a longer provisioning window.
func TestLDAPActiveDirectoryIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_LDAP_AD_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_LDAP_AD_INTEGRATION=1 to run against Samba AD-DC")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg := adIntegrationConfig(ctx, t)
	sess, err := connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	// Root DSE discovery should resolve the AD defaultNamingContext.
	if s.opts.BaseDN == "" {
		t.Fatal("AD base DN was not discovered")
	}

	ouDN := "ou=ShellCN," + adBaseDN
	userDN := "cn=Shellcn User," + ouDN

	mustAddEntry(ctx, t, s, adBaseDN, addEntryBody{RDN: "ou=ShellCN", ObjectClass: "top,organizationalUnit"})
	mustAddEntry(ctx, t, s, ouDN, addEntryBody{
		RDN:         "cn=Shellcn User",
		ObjectClass: "top,person,organizationalPerson,user",
		Attributes:  map[string][]string{"sAMAccountName": {"shellcnuser"}},
	})

	// The new OU shows up under the domain root.
	childRes, childErr := childEntries(rc(ctx, s, map[string]string{"dn": adBaseDN}, nil, nil))
	if !hasDN(t, childRes, childErr, ouDN) {
		t.Fatalf("ou=ShellCN missing from domain children")
	}

	// Attribute grid round-trip on an AD object.
	mutate(ctx, t, s, userDN, addAttribute, attrMutation{Values: map[string]any{"attribute": "description", "value": "created by shellcn"}})
	if got := attributeOf(ctx, t, s, userDN, "description"); got != "created by shellcn" {
		t.Fatalf("description after add = %q", got)
	}
	mutate(ctx, t, s, userDN, updateAttribute, attrMutation{Key: map[string]any{"attribute": "description"}, Values: map[string]any{"value": "edited by shellcn"}})
	if got := attributeOf(ctx, t, s, userDN, "description"); got != "edited by shellcn" {
		t.Fatalf("description after update = %q", got)
	}
	mutate(ctx, t, s, userDN, deleteAttribute, attrMutation{Key: map[string]any{"attribute": "description"}})
	if got := attributeOf(ctx, t, s, userDN, "description"); got != "" {
		t.Fatalf("description after delete = %q, want empty", got)
	}

	// Subtree search by AD attribute (sAMAccountName) finds the account.
	search := rc(ctx, s, map[string]string{"base": adBaseDN}, url.Values{"filter": {"(sAMAccountName=shellcnuser)"}}, nil)
	searchRes, searchErr := searchEntries(search)
	if !hasDN(t, searchRes, searchErr, userDN) {
		t.Fatalf("subtree search did not find %s", userDN)
	}

	// Rename the account within the same OU.
	renamedDN := "cn=Shellcn Renamed," + ouDN
	body, _ := json.Marshal(map[string]any{"new_rdn": "cn=Shellcn Renamed", "delete_old_rdn": true})
	if _, err := renameEntry(rc(ctx, s, map[string]string{"dn": userDN}, nil, body)); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := lookupEntry(s, renamedDN); err != nil {
		t.Fatalf("renamed entry missing: %v", err)
	}

	// Clean up: delete the account, then the (now empty) OU.
	if _, err := deleteEntry(rc(ctx, s, map[string]string{"dn": renamedDN}, nil, nil)); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := deleteEntry(rc(ctx, s, map[string]string{"dn": ouDN}, nil, nil)); err != nil {
		t.Fatalf("delete ou: %v", err)
	}
}

func adIntegrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	base := func(host string, port int) map[string]any {
		return map[string]any{
			"host": host, "port": port, "base_dn": adBaseDN, "encryption": encNone,
			"auth": authSimple, "bind_dn": adBindUPN, "password": adPwd, "read_only": false,
			"size_limit": 200, "page_size": 50,
		}
	}
	if addr := os.Getenv("SHELLCN_LDAP_AD_ADDR"); addr != "" {
		host, portText, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("parse SHELLCN_LDAP_AD_ADDR: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse AD port: %v", err)
		}
		return base(host, port)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_LDAP_AD_ADDR is not set")
	}
	name := "shellcn-ad-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name, "--privileged", "-h", "dc1",
		"-e", "DOMAIN="+adRealm, "-e", "DOMAINPASS="+adPwd,
		"-e", "INSECURELDAP=true", "-e", "NOCOMPLEXITY=true",
		"-p", "127.0.0.1::389", "nowsci/samba-domain:latest")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "389/tcp")
	host, portText, err := net.SplitHostPort(strings.TrimSpace(strings.Split(out, "\n")[0]))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", portText, err)
	}
	cfg := base(host, port)
	deadline := time.Now().Add(4 * time.Minute)
	for {
		sess, err := connect(ctx, plugin.ConnectConfig{
			Config: cfg,
			Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
		})
		if err == nil {
			_ = sess.Close()
			return cfg
		}
		if time.Now().After(deadline) {
			t.Fatalf("Samba AD-DC did not become ready: %v", err)
		}
		time.Sleep(2 * time.Second)
	}
}
