package proxmox

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// PVE lifecycle endpoints (clone, destroy, restore, resize, node power) return a
// UPID identifying the spawned task; callers track it through the tasks panel.
type taskResult struct {
	OK   bool   `json:"ok"`
	UPID string `json:"upid,omitempty"`
}

var (
	vmidRe  = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
	diskRe  = regexp.MustCompile(`^[a-z]+[0-9]+$`)
	sizeRe  = regexp.MustCompile(`^\+?[1-9][0-9]*[KMGT]?$`)
	nodeRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)
	storeRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	snapRe  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]+$`)
	upidRe  = regexp.MustCompile(`^UPID:[A-Za-z0-9.\-]+:[0-9A-Fa-f]+:[0-9A-Fa-f]+:[0-9A-Fa-f]+:[A-Za-z0-9_-]+:[^:]*:[^:]+:$`)
	powerOk = map[string]bool{"reboot": true, "shutdown": true}
)

func validVMID(s string) bool     { return vmidRe.MatchString(s) }
func validNode(s string) bool     { return nodeRe.MatchString(s) }
func validDisk(s string) bool     { return diskRe.MatchString(s) }
func validSize(s string) bool     { return sizeRe.MatchString(s) }
func validStorage(s string) bool  { return storeRe.MatchString(s) }
func validSnapName(s string) bool { return snapRe.MatchString(s) }
func validUPID(s string) bool     { return upidRe.MatchString(s) }

func validPowerCommand(s string) bool { return powerOk[s] }

func validBackupMode(s string) bool {
	return map[string]bool{"snapshot": true, "suspend": true, "stop": true}[s]
}

func validCompression(s string) bool {
	return map[string]bool{"zstd": true, "lzo": true, "gzip": true, "0": true}[s]
}

func validBackupVolume(storage, volume string) bool {
	if !validStorage(storage) {
		return false
	}
	volume = strings.TrimSpace(volume)
	if strings.ContainsAny(volume, "?#") {
		return false
	}
	return strings.HasPrefix(volume, storage+":backup/")
}

// post sends a body and decodes the PVE `data` field, which for lifecycle
// endpoints is the spawned task's UPID string.
func (s *Session) postUPID(ctx context.Context, path string, body any) (string, error) {
	var upid string
	if err := s.client.Post(ctx, path, body, &upid); err != nil {
		return "", mapErr(err)
	}
	return upid, nil
}

func (s *Session) putUPID(ctx context.Context, path string, body any) (string, error) {
	var upid string
	if err := s.client.Put(ctx, path, body, &upid); err != nil {
		return "", mapErr(err)
	}
	return upid, nil
}

func (s *Session) delUPID(ctx context.Context, path string) (string, error) {
	var upid string
	if err := s.client.Delete(ctx, path, &upid); err != nil {
		return "", mapErr(err)
	}
	return upid, nil
}

// cloneBody assembles the /clone request from the bound input, mapping the
// guest-name field to the kind's parameter (qemu: name, lxc: hostname).
func cloneBody(kind, newID, name, target, storage string, full bool) (map[string]any, error) {
	if !validVMID(newID) {
		return nil, fmt.Errorf("%w: target VMID must be a positive integer", plugin.ErrInvalidInput)
	}
	body := map[string]any{"newid": newID}
	if name = strings.TrimSpace(name); name != "" {
		if kind == "lxc" {
			body["hostname"] = name
		} else {
			body["name"] = name
		}
	}
	if target = strings.TrimSpace(target); target != "" {
		if !validNode(target) {
			return nil, fmt.Errorf("%w: invalid target node", plugin.ErrInvalidInput)
		}
		body["target"] = target
	}
	if storage = strings.TrimSpace(storage); storage != "" {
		if !validStorage(storage) {
			return nil, fmt.Errorf("%w: invalid storage", plugin.ErrInvalidInput)
		}
		body["storage"] = storage
	}
	if full {
		body["full"] = 1
	}
	return body, nil
}

func guestClone(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid := rc.Param("node"), rc.Param("vmid")
		if !validNode(node) || !validVMID(vmid) {
			return nil, fmt.Errorf("%w: invalid node or vmid", plugin.ErrInvalidInput)
		}
		var in struct {
			NewID   any    `json:"newid"`
			Name    string `json:"name"`
			Target  string `json:"target"`
			Storage string `json:"storage"`
			Full    bool   `json:"full"`
		}
		if err := rc.Bind(&in); err != nil {
			return nil, err
		}
		body, err := cloneBody(kind, bodyString(in.NewID), in.Name, in.Target, in.Storage, in.Full)
		if err != nil {
			return nil, err
		}
		upid, err := s.postUPID(rc.Ctx, pvePath("nodes", node, kind, vmid, "clone"), body)
		if err != nil {
			return nil, err
		}
		return taskResult{OK: true, UPID: upid}, nil
	}
}

func guestDestroy(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid := rc.Param("node"), rc.Param("vmid")
		if !validNode(node) || !validVMID(vmid) {
			return nil, fmt.Errorf("%w: invalid node or vmid", plugin.ErrInvalidInput)
		}
		path := pvePath("nodes", node, kind, vmid) + "?purge=1&destroy-unreferenced-disks=1"
		upid, err := s.delUPID(rc.Ctx, path)
		if err != nil {
			return nil, err
		}
		return taskResult{OK: true, UPID: upid}, nil
	}
}

// restoreBody builds the create-from-backup request. qemu restores via `archive`;
// lxc restores by passing the backup volume as `ostemplate` with `restore=1`.
func restoreBody(kind, vmid, archive, storage string, force bool) (map[string]any, error) {
	if !validVMID(vmid) {
		return nil, fmt.Errorf("%w: target VMID must be a positive integer", plugin.ErrInvalidInput)
	}
	if strings.TrimSpace(archive) == "" {
		return nil, fmt.Errorf("%w: backup archive is required", plugin.ErrInvalidInput)
	}
	body := map[string]any{"vmid": vmid}
	if kind == "lxc" {
		body["ostemplate"] = archive
		body["restore"] = 1
	} else {
		body["archive"] = archive
	}
	if storage = strings.TrimSpace(storage); storage != "" {
		if !validStorage(storage) {
			return nil, fmt.Errorf("%w: invalid storage", plugin.ErrInvalidInput)
		}
		body["storage"] = storage
	}
	if force {
		body["force"] = 1
	}
	return body, nil
}

func guestRestore(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node := rc.Param("node")
		if !validNode(node) {
			return nil, fmt.Errorf("%w: invalid node", plugin.ErrInvalidInput)
		}
		var in struct {
			VMID    any    `json:"vmid"`
			Archive string `json:"archive" validate:"required"`
			Storage string `json:"storage"`
			Force   bool   `json:"force"`
		}
		if err := rc.Bind(&in); err != nil {
			return nil, err
		}
		body, err := restoreBody(kind, bodyString(in.VMID), in.Archive, in.Storage, in.Force)
		if err != nil {
			return nil, err
		}
		upid, err := s.postUPID(rc.Ctx, pvePath("nodes", node, kind), body)
		if err != nil {
			return nil, err
		}
		return taskResult{OK: true, UPID: upid}, nil
	}
}

func qemuResize(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, vmid := rc.Param("node"), rc.Param("vmid")
	if !validNode(node) || !validVMID(vmid) {
		return nil, fmt.Errorf("%w: invalid node or vmid", plugin.ErrInvalidInput)
	}
	var in struct {
		Disk string `json:"disk" validate:"required"`
		Size string `json:"size" validate:"required"`
	}
	if err := rc.Bind(&in); err != nil {
		return nil, err
	}
	if !validDisk(in.Disk) {
		return nil, fmt.Errorf("%w: invalid disk identifier", plugin.ErrInvalidInput)
	}
	if !validSize(in.Size) {
		return nil, fmt.Errorf("%w: size must be like 50G or +10G", plugin.ErrInvalidInput)
	}
	body := map[string]any{"disk": in.Disk, "size": in.Size}
	upid, err := s.putUPID(rc.Ctx, pvePath("nodes", node, "qemu", vmid, "resize"), body)
	if err != nil {
		return nil, err
	}
	return taskResult{OK: true, UPID: upid}, nil
}

func nodePower(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node := rc.Param("node")
	if !validNode(node) {
		return nil, fmt.Errorf("%w: invalid node", plugin.ErrInvalidInput)
	}
	var in struct {
		Command string `json:"command" validate:"required"`
	}
	if err := rc.Bind(&in); err != nil {
		return nil, err
	}
	if !validPowerCommand(in.Command) {
		return nil, fmt.Errorf("%w: command must be reboot or shutdown", plugin.ErrInvalidInput)
	}
	if err := s.post(rc.Ctx, pvePath("nodes", node, "status"), map[string]any{"command": in.Command}); err != nil {
		return nil, err
	}
	return actionResult{OK: true}, nil
}

func taskStop(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, upid := rc.Param("node"), rc.Param("upid")
	if !validNode(node) || !validUPID(upid) {
		return nil, fmt.Errorf("%w: invalid node or task id", plugin.ErrInvalidInput)
	}
	if err := s.del(rc.Ctx, pvePath("nodes", node, "tasks", upid)); err != nil {
		return nil, err
	}
	return actionResult{OK: true}, nil
}

func taskStatus(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, upid := rc.Param("node"), rc.Param("upid")
	if !validNode(node) || !validUPID(upid) {
		return nil, fmt.Errorf("%w: invalid node or task id", plugin.ErrInvalidInput)
	}
	return s.object(rc.Ctx, pvePath("nodes", node, "tasks", upid, "status"))
}

func taskLog(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, upid := rc.Param("node"), rc.Param("upid")
	if !validNode(node) || !validUPID(upid) {
		return nil, fmt.Errorf("%w: invalid node or task id", plugin.ErrInvalidInput)
	}
	lines, err := s.list(rc.Ctx, pvePath("nodes", node, "tasks", upid, "log"))
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(lines))
	for _, l := range lines {
		rows = append(rows, row{"n": numInt(l["n"]), "t": str(l["t"])})
	}
	return pageRows(rc, rows)
}

// --- Input schemas --------------------------------------------------------

func cloneSchema(kind string) *plugin.Schema {
	nameLabel := "Name"
	if kind == "lxc" {
		nameLabel = "Hostname"
	}
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Clone", Fields: []plugin.Field{
		{Key: "newid", Label: "New VMID", Type: plugin.FieldNumber, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 100}, {Type: plugin.ValidatorMax, Value: 999999999}}},
		{Key: "name", Label: nameLabel, Type: plugin.FieldText},
		{Key: "target", Label: "Target node", Type: plugin.FieldSelect, OptionsSource: &plugin.DataSource{RouteID: "proxmox.node.options", Params: map[string]string{"node": "${resource.namespace}"}}, Help: "Leave empty to clone on the same node."},
		{Key: "storage", Label: "Target storage", Type: plugin.FieldText, Help: "Required for a full clone to another storage."},
		{Key: "full", Label: "Full clone", Type: plugin.FieldToggle, Help: "Copy all disks instead of a linked clone."},
	}}}}
}

func restoreSchema(kind string) *plugin.Schema {
	archiveHelp := "Backup volume id, e.g. local:backup/vzdump-qemu-100-....vma.zst"
	if kind == "lxc" {
		archiveHelp = "Backup volume id, e.g. local:backup/vzdump-lxc-100-....tar.zst"
	}
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Restore", Fields: []plugin.Field{
		{Key: "vmid", Label: "New VMID", Type: plugin.FieldNumber, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 100}, {Type: plugin.ValidatorMax, Value: 999999999}}},
		{Key: "archive", Label: "Backup archive", Type: plugin.FieldText, Required: true, Help: archiveHelp},
		{Key: "storage", Label: "Target storage", Type: plugin.FieldText},
		{Key: "force", Label: "Overwrite existing", Type: plugin.FieldToggle, Help: "Restore over an existing guest with this VMID."},
	}}}}
}

func resizeSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Resize disk", Fields: []plugin.Field{
		{Key: "disk", Label: "Disk", Type: plugin.FieldText, Required: true, Placeholder: "scsi0", Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[a-z]+[0-9]+$`, Message: "a disk identifier like scsi0 or virtio0"}}},
		{Key: "size", Label: "Size", Type: plugin.FieldText, Required: true, Placeholder: "+10G", Help: "Absolute (50G) or relative (+10G). Disks can only grow.", Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^\+?[1-9][0-9]*[KMGT]?$`, Message: "a size like 50G or +10G"}}},
	}}}}
}

func powerSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Power", Fields: []plugin.Field{
		{Key: "command", Label: "Action", Type: plugin.FieldSelect, Required: true, Options: []plugin.Option{
			{Label: "Reboot", Value: "reboot"},
			{Label: "Shutdown", Value: "shutdown"},
		}},
	}}}}
}
