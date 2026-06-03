package ldap

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

const (
	itBaseDN   = "dc=example,dc=org"
	itAdminDN  = "cn=admin,dc=example,dc=org"
	itAdminPwd = "adminpass"
)

func TestLDAPPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_LDAP_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_LDAP_INTEGRATION=1 to run against OpenLDAP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg := integrationConfig(ctx, t)
	sess, err := connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	// Tree root resolves to the configured base DN.
	rootRes, err := treeRoot(rc(ctx, s, nil, nil, nil))
	if err != nil {
		t.Fatalf("tree root: %v", err)
	}
	root := rootRes.(plugin.Page[plugin.TreeNode])
	if len(root.Items) != 1 || root.Items[0].Key != itBaseDN {
		t.Fatalf("unexpected tree root: %#v", root.Items)
	}

	peopleDN := "ou=people," + itBaseDN
	personDN := "uid=jdoe," + peopleDN

	// Add an organizational unit, then a person beneath it.
	mustAddEntry(ctx, t, s, itBaseDN, addEntryBody{RDN: "ou=people", ObjectClass: "top,organizationalUnit"})
	mustAddEntry(ctx, t, s, peopleDN, addEntryBody{
		RDN:         "uid=jdoe",
		ObjectClass: "top,person,organizationalPerson,inetOrgPerson",
		Attributes:  map[string][]string{"cn": {"John Doe"}, "sn": {"Doe"}},
	})

	// Children of the base include the new OU.
	childRes, childErr := childEntries(rc(ctx, s, map[string]string{"dn": itBaseDN}, nil, nil))
	if !hasDN(t, childRes, childErr, peopleDN) {
		t.Fatalf("ou=people missing from base children")
	}

	// Attribute grid round-trip: add, update, delete a multi-valued attribute.
	mutate(ctx, t, s, personDN, addAttribute, attrMutation{Values: map[string]any{"attribute": "mail", "value": "jdoe@example.org"}})
	if got := attributeOf(ctx, t, s, personDN, "mail"); got != "jdoe@example.org" {
		t.Fatalf("mail after add = %q", got)
	}
	mutate(ctx, t, s, personDN, updateAttribute, attrMutation{Key: map[string]any{"attribute": "mail"}, Values: map[string]any{"value": "john@example.org"}})
	if got := attributeOf(ctx, t, s, personDN, "mail"); got != "john@example.org" {
		t.Fatalf("mail after update = %q", got)
	}
	mutate(ctx, t, s, personDN, deleteAttribute, attrMutation{Key: map[string]any{"attribute": "mail"}})
	if got := attributeOf(ctx, t, s, personDN, "mail"); got != "" {
		t.Fatalf("mail after delete = %q, want empty", got)
	}

	// Subtree search by raw filter finds the person.
	search := rc(ctx, s, map[string]string{"base": itBaseDN}, url.Values{"filter": {"(uid=jdoe)"}}, nil)
	searchRes, searchErr := searchEntries(search)
	if !hasDN(t, searchRes, searchErr, personDN) {
		t.Fatalf("subtree search did not find %s", personDN)
	}

	// Rename within the same parent.
	renamedDN := "uid=johndoe," + peopleDN
	body, _ := json.Marshal(map[string]any{"new_rdn": "uid=johndoe", "delete_old_rdn": true})
	if _, err := renameEntry(rc(ctx, s, map[string]string{"dn": personDN}, nil, body)); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := lookupEntry(s, renamedDN); err != nil {
		t.Fatalf("renamed entry missing: %v", err)
	}

	// Delete the renamed entry and its parent OU.
	if _, err := deleteEntry(rc(ctx, s, map[string]string{"dn": renamedDN}, nil, nil)); err != nil {
		t.Fatalf("delete person: %v", err)
	}
	if _, err := deleteEntry(rc(ctx, s, map[string]string{"dn": peopleDN}, nil, nil)); err != nil {
		t.Fatalf("delete ou: %v", err)
	}

	// Read-only sessions reject writes before touching the server.
	if err := ensureWritable(&Session{opts: options{ReadOnly: true}}); err == nil {
		t.Fatal("read-only session should block writes")
	}
}

type addEntryBody struct {
	RDN         string              `json:"rdn"`
	ObjectClass string              `json:"object_class"`
	Attributes  map[string][]string `json:"attributes,omitempty"`
}

func mustAddEntry(ctx context.Context, t *testing.T, s *Session, parent string, body addEntryBody) {
	t.Helper()
	raw, _ := json.Marshal(body)
	if _, err := addEntry(rc(ctx, s, map[string]string{"parent": parent}, nil, raw)); err != nil {
		t.Fatalf("add entry %s under %s: %v", body.RDN, parent, err)
	}
}

func mutate(ctx context.Context, t *testing.T, s *Session, dn string, handler func(*plugin.RequestContext) (any, error), m attrMutation) {
	t.Helper()
	raw, _ := json.Marshal(m)
	if _, err := handler(rc(ctx, s, map[string]string{"dn": dn}, nil, raw)); err != nil {
		t.Fatalf("attribute mutation on %s: %v", dn, err)
	}
}

func attributeOf(ctx context.Context, t *testing.T, s *Session, dn, attr string) string {
	t.Helper()
	res, err := entryAttributes(rc(ctx, s, map[string]string{"dn": dn}, nil, nil))
	if err != nil {
		t.Fatalf("read attributes of %s: %v", dn, err)
	}
	for _, r := range res.(plugin.Page[row]).Items {
		if r["attribute"] == attr {
			return strings.Trim(asString(r["value"]), "[]\"")
		}
	}
	return ""
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func hasDN(t *testing.T, res any, err error, dn string) bool {
	t.Helper()
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	page, ok := res.(plugin.Page[row])
	if !ok {
		t.Fatalf("expected Page[row], got %T", res)
	}
	for _, r := range page.Items {
		// LDAP DNs are case-insensitive; AD normalizes attribute-type casing
		// (e.g. DC=, OU=), so compare without regard to case.
		if strings.EqualFold(fmt.Sprint(r["dn"]), dn) {
			return true
		}
	}
	return false
}

func rc(ctx context.Context, s *Session, params map[string]string, query url.Values, body []byte) *plugin.RequestContext {
	return plugin.NewRequestContext(ctx, plugin.User{}, s, params, query, body)
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	base := func(host string, port int) map[string]any {
		return map[string]any{
			"host": host, "port": port, "base_dn": itBaseDN, "encryption": encNone,
			"auth": authSimple, "bind_dn": itAdminDN, "password": itAdminPwd, "read_only": false,
		}
	}
	if addr := os.Getenv("SHELLCN_LDAP_ADDR"); addr != "" {
		host, portText, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("parse SHELLCN_LDAP_ADDR: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse LDAP port: %v", err)
		}
		return base(host, port)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_LDAP_ADDR is not set")
	}
	name := "shellcn-ldap-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "LDAP_ORGANISATION=ShellCN", "-e", "LDAP_DOMAIN=example.org",
		"-e", "LDAP_ADMIN_PASSWORD="+itAdminPwd,
		"-p", "127.0.0.1::389", "osixia/openldap:1.5.0")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	deadline := time.Now().Add(60 * time.Second)
	for {
		sess, err := connect(ctx, plugin.ConnectConfig{
			Config: cfg,
			Net:    plugintest.DirectTransport(),
		})
		if err == nil {
			_ = sess.Close()
			return cfg
		}
		if time.Now().After(deadline) {
			t.Fatalf("LDAP container did not become ready: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func run(ctx context.Context, t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}
