package ldap

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type row map[string]any

type actionResult struct {
	OK bool `json:"ok"`
}

// attrMutation is the uniform body the editable grid sends for attribute
// changes: insert {values:{attribute,value}}, update {key:{attribute},
// values:{value}}, delete {key:{attribute}}.
type attrMutation struct {
	Key    map[string]any `json:"key"`
	Values map[string]any `json:"values"`
}

func routes() []plugin.Route {
	return []plugin.Route{
		{ID: "ldap.tree.root", Method: plugin.MethodGet, Path: "/tree/root", Permission: "ldap.entries.read", Risk: plugin.RiskSafe, AuditEvent: "ldap.tree.root", Handle: treeRoot},
		{ID: "ldap.tree.children", Method: plugin.MethodGet, Path: "/tree/children", Permission: "ldap.entries.read", Risk: plugin.RiskSafe, AuditEvent: "ldap.tree.children", Handle: treeChildren},

		{ID: "ldap.entries.search", Method: plugin.MethodGet, Path: "/entries", Permission: "ldap.entries.read", Risk: plugin.RiskSafe, AuditEvent: "ldap.entries.search", Handle: searchEntries},
		{ID: "ldap.entry.children", Method: plugin.MethodGet, Path: "/entries/children", Permission: "ldap.entries.read", Risk: plugin.RiskSafe, AuditEvent: "ldap.entry.children", Handle: childEntries},
		{ID: "ldap.entry.attributes", Method: plugin.MethodGet, Path: "/entries/attributes", Permission: "ldap.entries.read", Risk: plugin.RiskSafe, AuditEvent: "ldap.entry.attributes", Handle: entryAttributes},
		{ID: "ldap.entry.ldif", Method: plugin.MethodGet, Path: "/entries/ldif", Permission: "ldap.entries.read", Risk: plugin.RiskSafe, AuditEvent: "ldap.entry.ldif", Handle: entryLDIF},

		{ID: "ldap.entry.attr.add", Method: plugin.MethodPost, Path: "/entries/attributes/add", Permission: "ldap.entries.write", Risk: plugin.RiskWrite, AuditEvent: "ldap.entry.attr.add", Handle: addAttribute},
		{ID: "ldap.entry.attr.update", Method: plugin.MethodPatch, Path: "/entries/attributes", Permission: "ldap.entries.write", Risk: plugin.RiskWrite, AuditEvent: "ldap.entry.attr.update", Handle: updateAttribute},
		{ID: "ldap.entry.attr.delete", Method: plugin.MethodDelete, Path: "/entries/attributes", Permission: "ldap.entries.write", Risk: plugin.RiskDestructive, AuditEvent: "ldap.entry.attr.delete", Handle: deleteAttribute},

		{ID: "ldap.entry.add", Method: plugin.MethodPost, Path: "/entries", Permission: "ldap.entries.write", Risk: plugin.RiskWrite, AuditEvent: "ldap.entry.add", Input: entryAddSchema(), Handle: addEntry},
		{ID: "ldap.entry.rename", Method: plugin.MethodPost, Path: "/entries/rename", Permission: "ldap.entries.write", Risk: plugin.RiskWrite, AuditEvent: "ldap.entry.rename", Input: entryRenameSchema(), Handle: renameEntry},
		{ID: "ldap.entry.delete", Method: plugin.MethodDelete, Path: "/entries", Permission: "ldap.entries.delete", Risk: plugin.RiskDestructive, AuditEvent: "ldap.entry.delete", Handle: deleteEntry},
	}
}

func entryAddSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Entry", Fields: []plugin.Field{
		{Key: "rdn", Label: "RDN", Type: plugin.FieldText, Required: true, Placeholder: "uid=jdoe", Help: "Relative DN of the new entry, e.g. uid=jdoe or cn=Engineers."},
		{Key: "object_class", Label: "Object classes", Type: plugin.FieldText, Required: true, Default: "top", Help: "Comma-separated objectClass values, e.g. top,inetOrgPerson."},
		{Key: "attributes", Label: "Attributes", Type: plugin.FieldJSON, Help: `Optional JSON of attribute values, e.g. {"cn":["John Doe"],"sn":["Doe"]}.`},
	}}}}
}

func entryRenameSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Rename / move", Fields: []plugin.Field{
		{Key: "new_rdn", Label: "New RDN", Type: plugin.FieldText, Required: true, Placeholder: "uid=johndoe", Help: "New relative DN for the entry."},
		{Key: "new_superior", Label: "New parent DN", Type: plugin.FieldText, Help: "Move the entry under this parent DN. Leave empty to rename in place."},
		{Key: "delete_old_rdn", Label: "Delete old RDN", Type: plugin.FieldToggle, Default: true, Help: "Remove the previous RDN attribute value after the rename."},
	}}}}
}

// --- session helpers ----------------------------------------------------

func ldapSession(rc *plugin.RequestContext) (*Session, error) {
	s, err := unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	return s, nil
}

// paramOf reads a value that may arrive as a path param or as a plain query
// param (p.name) when the route template does not declare it.
func paramOf(rc *plugin.RequestContext, name string) string {
	if v := rc.Param(name); v != "" {
		return v
	}
	return rc.Query().Get("p." + name)
}

func dnParam(rc *plugin.RequestContext) (string, error) {
	dn := strings.TrimSpace(paramOf(rc, "dn"))
	if dn == "" {
		return "", fmt.Errorf("%w: dn is required", plugin.ErrInvalidInput)
	}
	return dn, nil
}

func ensureWritable(s *Session) error {
	if s.opts.ReadOnly {
		return fmt.Errorf("%w: read-only mode blocks write operations", plugin.ErrForbidden)
	}
	return nil
}

// --- tree ---------------------------------------------------------------

func treeRoot(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	entry, err := lookupEntry(s, s.opts.BaseDN)
	if err != nil {
		return nil, err
	}
	node := treeNode(entry)
	total := 1
	return plugin.Page[plugin.TreeNode]{Items: []plugin.TreeNode{node}, Total: &total}, nil
}

func treeChildren(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	dn, err := dnParam(rc)
	if err != nil {
		return nil, err
	}
	entries, err := search(s, dn, ldapv3.ScopeSingleLevel, "(objectClass=*)", structAttrs)
	if err != nil {
		return nil, err
	}
	nodes := make([]plugin.TreeNode, 0, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, treeNode(entry))
	}
	total := len(nodes)
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: &total}, nil
}

// --- entry lists --------------------------------------------------------

func searchEntries(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(paramOf(rc, "base"))
	if base == "" {
		base = s.opts.BaseDN
	}
	filter := searchFilter(req.Search())
	entries, err := search(s, base, ldapv3.ScopeWholeSubtree, filter, structAttrs)
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, entryRow(entry))
	}
	return paginate(rc, rows)
}

func childEntries(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	dn, err := dnParam(rc)
	if err != nil {
		return nil, err
	}
	entries, err := search(s, dn, ldapv3.ScopeSingleLevel, "(objectClass=*)", structAttrs)
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, entryRow(entry))
	}
	return pageRows(rc, rows)
}

func entryAttributes(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	dn, err := dnParam(rc)
	if err != nil {
		return nil, err
	}
	entry, err := lookupEntry(s, dn)
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(entry.Attributes))
	for _, attr := range entry.Attributes {
		rows = append(rows, row{"attribute": attr.Name, "value": attributeValue(attr.Values)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return fmt.Sprint(rows[i]["attribute"]) < fmt.Sprint(rows[j]["attribute"])
	})
	return pageRows(rc, rows)
}

func entryLDIF(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	dn, err := dnParam(rc)
	if err != nil {
		return nil, err
	}
	entry, err := lookupEntry(s, dn)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "dn: %s\n", entry.DN)
	for _, attr := range entry.Attributes {
		for _, value := range attr.Values {
			fmt.Fprintf(&b, "%s: %s\n", attr.Name, value)
		}
	}
	return row{"name": rdnOf(entry.DN), "dn": entry.DN, "definition": b.String()}, nil
}

// --- attribute mutations ------------------------------------------------

func addAttribute(rc *plugin.RequestContext) (any, error) {
	s, dn, m, err := mutationContext(rc)
	if err != nil {
		return nil, err
	}
	attr := strings.TrimSpace(fmt.Sprint(m.Values["attribute"]))
	if attr == "" {
		return nil, fmt.Errorf("%w: attribute name is required", plugin.ErrInvalidInput)
	}
	values := attributeValues(m.Values["value"])
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: a value is required to add an attribute", plugin.ErrInvalidInput)
	}
	req := ldapv3.NewModifyRequest(dn, nil)
	req.Add(attr, values)
	if err := s.conn.Modify(req); err != nil {
		return nil, ldapErr(err)
	}
	return actionResult{OK: true}, nil
}

func updateAttribute(rc *plugin.RequestContext) (any, error) {
	s, dn, m, err := mutationContext(rc)
	if err != nil {
		return nil, err
	}
	attr, err := keyAttribute(m.Key)
	if err != nil {
		return nil, err
	}
	values := attributeValues(m.Values["value"])
	req := ldapv3.NewModifyRequest(dn, nil)
	if len(values) == 0 {
		req.Delete(attr, nil)
	} else {
		req.Replace(attr, values)
	}
	if err := s.conn.Modify(req); err != nil {
		return nil, ldapErr(err)
	}
	return actionResult{OK: true}, nil
}

func deleteAttribute(rc *plugin.RequestContext) (any, error) {
	s, dn, m, err := mutationContext(rc)
	if err != nil {
		return nil, err
	}
	attr, err := keyAttribute(m.Key)
	if err != nil {
		return nil, err
	}
	req := ldapv3.NewModifyRequest(dn, nil)
	req.Delete(attr, nil)
	if err := s.conn.Modify(req); err != nil {
		return nil, ldapErr(err)
	}
	return actionResult{OK: true}, nil
}

func mutationContext(rc *plugin.RequestContext) (*Session, string, attrMutation, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, "", attrMutation{}, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, "", attrMutation{}, err
	}
	dn, err := dnParam(rc)
	if err != nil {
		return nil, "", attrMutation{}, err
	}
	var m attrMutation
	if err := rc.Bind(&m); err != nil {
		return nil, "", attrMutation{}, err
	}
	return s, dn, m, nil
}

func keyAttribute(key map[string]any) (string, error) {
	attr := strings.TrimSpace(fmt.Sprint(key["attribute"]))
	if attr == "" {
		return "", fmt.Errorf("%w: attribute key is required", plugin.ErrInvalidInput)
	}
	return attr, nil
}

// --- entry lifecycle ----------------------------------------------------

func addEntry(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	parent := strings.TrimSpace(paramOf(rc, "parent"))
	if parent == "" {
		parent = s.opts.BaseDN
	}
	var req struct {
		RDN         string              `json:"rdn" validate:"required"`
		ObjectClass string              `json:"object_class" validate:"required"`
		Attributes  map[string][]string `json:"attributes"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	rdn := strings.TrimSpace(req.RDN)
	attrName, attrValue, ok := strings.Cut(rdn, "=")
	if !ok || strings.TrimSpace(attrName) == "" || strings.TrimSpace(attrValue) == "" {
		return nil, fmt.Errorf("%w: RDN must be in attribute=value form", plugin.ErrInvalidInput)
	}
	add := ldapv3.NewAddRequest(rdn+","+parent, nil)
	add.Attribute("objectClass", splitList(req.ObjectClass))
	merged := map[string][]string{}
	for name, values := range req.Attributes {
		merged[strings.ToLower(name)] = values
	}
	for name, values := range req.Attributes {
		if strings.EqualFold(name, "objectClass") {
			continue
		}
		add.Attribute(name, values)
	}
	if _, exists := merged[strings.ToLower(attrName)]; !exists {
		add.Attribute(strings.TrimSpace(attrName), []string{strings.TrimSpace(attrValue)})
	}
	if err := s.conn.Add(add); err != nil {
		return nil, ldapErr(err)
	}
	return actionResult{OK: true}, nil
}

func renameEntry(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	dn, err := dnParam(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		NewRDN       string `json:"new_rdn" validate:"required"`
		NewSuperior  string `json:"new_superior"`
		DeleteOldRDN bool   `json:"delete_old_rdn"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	move := ldapv3.NewModifyDNRequest(dn, strings.TrimSpace(req.NewRDN), req.DeleteOldRDN, strings.TrimSpace(req.NewSuperior))
	if err := s.conn.ModifyDN(move); err != nil {
		return nil, ldapErr(err)
	}
	return actionResult{OK: true}, nil
}

func deleteEntry(rc *plugin.RequestContext) (any, error) {
	s, err := ldapSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	dn, err := dnParam(rc)
	if err != nil {
		return nil, err
	}
	if err := s.conn.Del(ldapv3.NewDelRequest(dn, nil)); err != nil {
		return nil, ldapErr(err)
	}
	return actionResult{OK: true}, nil
}

// --- LDAP helpers -------------------------------------------------------

// structAttrs are the lightweight attributes needed to render tree nodes and
// table rows without fetching every value of every entry.
var structAttrs = []string{"objectClass", "hasSubordinates"}

// search runs a paged LDAP search bounded by the connection's size limit. The
// Simple Paged Results control lets it page past per-request server caps (e.g.
// Active Directory's default MaxPageSize of 1000); the control is non-critical,
// so directories that don't support it just return one bounded page. Paging
// stops once the size limit is reached.
func search(s *Session, base string, scope int, filter string, attrs []string) ([]*ldapv3.Entry, error) {
	pageSize := s.opts.PageSize
	if pageSize <= 0 || pageSize > s.opts.SizeLimit {
		pageSize = s.opts.SizeLimit
	}
	timeout := int(s.opts.Timeout.Seconds())
	paging := ldapv3.NewControlPaging(uint32(pageSize))
	var entries []*ldapv3.Entry
	for {
		req := ldapv3.NewSearchRequest(base, scope, ldapv3.NeverDerefAliases, s.opts.SizeLimit, timeout, false, filter, attrs, []ldapv3.Control{paging})
		res, err := s.conn.Search(req)
		if res != nil {
			entries = append(entries, res.Entries...)
		}
		if err != nil {
			// A size-limit cut-off still yields the entries gathered so far.
			if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultSizeLimitExceeded) {
				break
			}
			return nil, ldapErr(err)
		}
		if len(entries) >= s.opts.SizeLimit {
			abandonPaging(s, base, scope, filter, attrs, timeout, res)
			break
		}
		cookie := pagingCookie(res)
		if len(cookie) == 0 {
			break
		}
		paging.SetCookie(cookie)
	}
	if len(entries) > s.opts.SizeLimit {
		entries = entries[:s.opts.SizeLimit]
	}
	return entries, nil
}

func pagingCookie(res *ldapv3.SearchResult) []byte {
	if res == nil {
		return nil
	}
	if ctrl, ok := ldapv3.FindControl(res.Controls, ldapv3.ControlTypePaging).(*ldapv3.ControlPaging); ok && ctrl != nil {
		return ctrl.Cookie
	}
	return nil
}

// abandonPaging best-effort releases a paged-search cursor we stopped consuming
// early (a final request with page size 0), so the server doesn't keep it open.
func abandonPaging(s *Session, base string, scope int, filter string, attrs []string, timeout int, res *ldapv3.SearchResult) {
	cookie := pagingCookie(res)
	if len(cookie) == 0 {
		return
	}
	stop := ldapv3.NewControlPaging(0)
	stop.SetCookie(cookie)
	req := ldapv3.NewSearchRequest(base, scope, ldapv3.NeverDerefAliases, 1, timeout, false, filter, attrs, []ldapv3.Control{stop})
	_, _ = s.conn.Search(req)
}

func lookupEntry(s *Session, dn string) (*ldapv3.Entry, error) {
	req := ldapv3.NewSearchRequest(dn, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, int(s.opts.Timeout.Seconds()), false, "(objectClass=*)", []string{"*", "hasSubordinates"}, nil)
	res, err := s.conn.Search(req)
	if err != nil {
		return nil, ldapErr(err)
	}
	if len(res.Entries) == 0 {
		return nil, plugin.ErrNotFound
	}
	return res.Entries[0], nil
}

func treeNode(entry *ldapv3.Entry) plugin.TreeNode {
	classes := entry.GetAttributeValues("objectClass")
	leaf := strings.EqualFold(entry.GetAttributeValue("hasSubordinates"), "FALSE")
	node := plugin.TreeNode{
		Key:   entry.DN,
		Label: rdnOf(entry.DN),
		Icon:  iconForEntry(classes),
		Ref:   entryRef(entry.DN),
		Leaf:  leaf,
	}
	if !leaf {
		node.ChildrenSource = &plugin.DataSource{RouteID: "ldap.tree.children", Params: map[string]string{"dn": entry.DN}}
	}
	return node
}

func entryRow(entry *ldapv3.Entry) row {
	ref := entryRef(entry.DN)
	return row{
		"name":        ref.Name,
		"dn":          entry.DN,
		"objectClass": strings.Join(entry.GetAttributeValues("objectClass"), ", "),
		"ref":         *ref,
	}
}

func entryRef(dn string) *plugin.ResourceRef {
	return &plugin.ResourceRef{Kind: "entry", Name: rdnOf(dn), Namespace: parentOf(dn), UID: dn}
}

func iconForEntry(classes []string) plugin.Icon {
	for _, class := range classes {
		switch strings.ToLower(class) {
		case "organizationalunit", "organization", "domain", "domaindns", "builtindomain", "dcobject", "container":
			return icon("folder")
		case "groupofnames", "groupofuniquenames", "posixgroup", "group":
			return icon("users")
		case "person", "inetorgperson", "organizationalperson", "user", "posixaccount", "foreignsecurityprincipal":
			return icon("user")
		case "computer", "device":
			return icon("monitor")
		}
	}
	return icon("file")
}

func rdnOf(dn string) string {
	dn = strings.TrimSpace(dn)
	if dn == "" {
		return dn
	}
	return strings.SplitN(dn, ",", 2)[0]
}

func parentOf(dn string) string {
	parts := strings.SplitN(strings.TrimSpace(dn), ",", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func searchFilter(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return "(objectClass=*)"
	}
	if strings.HasPrefix(q, "(") {
		return q
	}
	esc := ldapv3.EscapeFilter(q)
	return fmt.Sprintf("(|(cn=*%s*)(uid=*%s*)(ou=*%s*)(mail=*%s*)(sn=*%s*))", esc, esc, esc, esc, esc)
}

// attributeValue collapses an attribute's values for the grid cell: a single
// value renders as text; multiple render as a JSON array so the cell stays
// round-trippable through the editor.
func attributeValue(values []string) any {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	default:
		raw, err := json.Marshal(values)
		if err != nil {
			return strings.Join(values, ", ")
		}
		return string(raw)
	}
}

// attributeValues parses a grid cell back into LDAP attribute values, accepting
// a JSON array, a JSON-encoded list, or a single scalar.
func attributeValues(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case []string:
		return t
	case string:
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			return nil
		}
		if strings.HasPrefix(trimmed, "[") {
			var parsed []string
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
				return parsed
			}
		}
		return []string{t}
	default:
		return []string{fmt.Sprint(v)}
	}
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ldapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject):
		return plugin.ErrNotFound
	case ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultEntryAlreadyExists):
		return fmt.Errorf("%w: entry already exists", plugin.ErrAlreadyExists)
	case ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultInsufficientAccessRights):
		return fmt.Errorf("%w: insufficient access rights", plugin.ErrForbidden)
	default:
		return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
	}
}

// --- paging -------------------------------------------------------------

func paginate(rc *plugin.RequestContext, rows []row) (plugin.Page[row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[row]{}, err
	}
	sortRows(rows, req.Sort)
	return sliceRows(rows, req)
}

func pageRows(rc *plugin.RequestContext, rows []row) (plugin.Page[row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[row]{}, err
	}
	rows = filterRows(rows, req.Search())
	sortRows(rows, req.Sort)
	return sliceRows(rows, req)
}

func sliceRows(rows []row, req plugin.PageRequest) (plugin.Page[row], error) {
	total := len(rows)
	start := 0
	if req.Cursor != "" {
		n, err := strconv.Atoi(req.Cursor)
		if err != nil || n < 0 {
			return plugin.Page[row]{}, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
		}
		start = n
	}
	if start > total {
		start = total
	}
	end := min(start+req.Limit, total)
	next := ""
	if end < total {
		next = strconv.Itoa(end)
	}
	return plugin.Page[row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func filterRows(rows []row, q string) []row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := make([]row, 0, len(rows))
	for _, r := range rows {
		for _, value := range r {
			if strings.Contains(strings.ToLower(fmt.Sprint(value)), q) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func sortRows(rows []row, keys []plugin.SortKey) {
	if len(keys) == 0 {
		return
	}
	key := keys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := fmt.Sprint(rows[i][key.Field]), fmt.Sprint(rows[j][key.Field])
		if key.Desc {
			return a > b
		}
		return a < b
	})
}
