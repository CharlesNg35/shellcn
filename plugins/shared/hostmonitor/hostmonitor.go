// Package hostmonitor collects operating-system inventory and metrics for the
// server monitor plugin and the agent's remote host-monitor mode.
package hostmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

type Row map[string]any

type Backend interface {
	Overview(context.Context) (map[string]any, error)
	Metrics(context.Context) (map[string]any, error)
	Processes(context.Context) ([]Row, error)
	Services(context.Context) ([]Row, error)
	Disks(context.Context) ([]Row, error)
	DiskIO(context.Context) ([]Row, error)
	Networks(context.Context) ([]Row, error)
	Connections(context.Context) ([]Row, error)
	Users(context.Context) ([]Row, error)
	Sensors(context.Context) ([]Row, error)
	CPUInfo(context.Context) ([]Row, error)
}

type Options struct {
	ProcessLimit    int
	ConnectionLimit int
}

type Local struct {
	opts Options
}

func NewLocal(opts Options) *Local {
	if opts.ProcessLimit <= 0 {
		opts.ProcessLimit = 1000
	}
	if opts.ConnectionLimit <= 0 {
		opts.ConnectionLimit = 1000
	}
	return &Local{opts: opts}
}

func (l *Local) Overview(ctx context.Context) (map[string]any, error) {
	out := map[string]any{
		"os":         runtime.GOOS,
		"kernelArch": runtime.GOARCH,
	}
	if info, err := host.InfoWithContext(ctx); err == nil {
		out["hostname"] = info.Hostname
		out["os"] = info.OS
		out["platform"] = info.Platform
		out["platformFamily"] = info.PlatformFamily
		out["platformVersion"] = info.PlatformVersion
		out["kernelVersion"] = info.KernelVersion
		out["kernelArch"] = info.KernelArch
		out["uptimeSeconds"] = info.Uptime
		out["bootTime"] = unixSeconds(info.BootTime)
		out["virtualizationSystem"] = info.VirtualizationSystem
		out["virtualizationRole"] = info.VirtualizationRole
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil {
		out["cpuCores"] = len(infos)
		if len(infos) > 0 {
			out["cpuModel"] = infos[0].ModelName
			out["cpuMhz"] = infos[0].Mhz
			out["cpuVendor"] = infos[0].VendorID
			out["cpuFamily"] = infos[0].Family
			out["cpuStepping"] = infos[0].Stepping
		}
	}
	if pct, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(pct) > 0 {
		out["cpuPct"] = pct[0]
	}
	if pct, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		out["cpuPerCore"] = pct
	}
	if avg, err := load.AvgWithContext(ctx); err == nil {
		out["load1"] = avg.Load1
		out["load5"] = avg.Load5
		out["load15"] = avg.Load15
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		out["memTotal"] = vm.Total
		out["memAvailable"] = vm.Available
		out["memUsed"] = vm.Used
		out["memPct"] = vm.UsedPercent
	}
	if sm, err := mem.SwapMemoryWithContext(ctx); err == nil {
		out["swapTotal"] = sm.Total
		out["swapUsed"] = sm.Used
		out["swapPct"] = sm.UsedPercent
	}
	if procs, err := process.ProcessesWithContext(ctx); err == nil {
		out["processes"] = len(procs)
	}
	if users, err := host.UsersWithContext(ctx); err == nil {
		out["sessions"] = len(users)
	}
	return out, nil
}

func (l *Local) Metrics(ctx context.Context) (map[string]any, error) {
	out := map[string]any{}
	if pct, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(pct) > 0 {
		out["cpuPct"] = pct[0]
	}
	if avg, err := load.AvgWithContext(ctx); err == nil {
		out["load1"] = avg.Load1
		out["load5"] = avg.Load5
		out["load15"] = avg.Load15
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		out["memPct"] = vm.UsedPercent
		out["memUsed"] = vm.Used
		out["memTotal"] = vm.Total
	}
	if sm, err := mem.SwapMemoryWithContext(ctx); err == nil {
		out["swapPct"] = sm.UsedPercent
		out["swapUsed"] = sm.Used
	}
	if ios, err := gnet.IOCountersWithContext(ctx, false); err == nil && len(ios) > 0 {
		out["netBytesRecv"] = ios[0].BytesRecv
		out["netBytesSent"] = ios[0].BytesSent
	}
	if ios, err := disk.IOCountersWithContext(ctx); err == nil {
		var read, written uint64
		for _, io := range ios {
			read += io.ReadBytes
			written += io.WriteBytes
		}
		out["diskReadBytes"] = read
		out["diskWriteBytes"] = written
	}
	if procs, err := process.ProcessesWithContext(ctx); err == nil {
		out["processes"] = len(procs)
	}
	if users, err := host.UsersWithContext(ctx); err == nil {
		out["sessions"] = len(users)
	}
	out["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	return out, nil
}

func (l *Local) Processes(ctx context.Context) ([]Row, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return []Row{}, nil
	}
	rows := make([]Row, 0, len(procs))
	for _, p := range procs {
		name, _ := p.NameWithContext(ctx)
		status, _ := p.StatusWithContext(ctx)
		user, _ := p.UsernameWithContext(ctx)
		cmdline, _ := p.CmdlineWithContext(ctx)
		created, _ := p.CreateTimeWithContext(ctx)
		threads, _ := p.NumThreadsWithContext(ctx)
		memPct, _ := p.MemoryPercentWithContext(ctx)
		cpuPct, _ := p.CPUPercentWithContext(ctx)
		var rss, vms uint64
		if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
			rss, vms = mi.RSS, mi.VMS
		}
		rows = append(rows, Row{
			"ref":       ref("process", strconv.Itoa(int(p.Pid)), name),
			"pid":       p.Pid,
			"name":      name,
			"status":    strings.Join(status, ","),
			"user":      user,
			"cpuPct":    cpuPct,
			"memPct":    memPct,
			"rss":       rss,
			"vms":       vms,
			"threads":   threads,
			"createdAt": unixMillis(created),
			"cmdline":   cmdline,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return numeric(rows[i]["rss"]) > numeric(rows[j]["rss"])
	})
	if l.opts.ProcessLimit > 0 && len(rows) > l.opts.ProcessLimit {
		rows = rows[:l.opts.ProcessLimit]
	}
	return rows, nil
}

func (l *Local) Services(ctx context.Context) ([]Row, error) {
	switch runtime.GOOS {
	case "linux":
		return linuxServices(ctx)
	case "darwin":
		return darwinServices(ctx)
	case "windows":
		return windowsServices(ctx)
	default:
		return []Row{}, nil
	}
}

func linuxServices(ctx context.Context) ([]Row, error) {
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return []Row{}, nil
	}
	cmd := exec.CommandContext(ctx, path, "list-units", "--type=service", "--all", "--no-legend", "--no-pager", "--plain")
	out, err := cmd.Output()
	if err != nil {
		return []Row{}, nil
	}
	lines := strings.Split(string(out), "\n")
	rows := make([]Row, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		}
		rows = append(rows, Row{
			"ref":         ref("service", fields[0], fields[0]),
			"unit":        fields[0],
			"name":        strings.TrimSuffix(fields[0], ".service"),
			"load":        fields[1],
			"active":      fields[2],
			"sub":         fields[3],
			"description": desc,
		})
	}
	return rows, nil
}

func darwinServices(ctx context.Context) ([]Row, error) {
	path, err := exec.LookPath("launchctl")
	if err != nil {
		return []Row{}, nil
	}
	out, err := exec.CommandContext(ctx, path, "list").Output()
	if err != nil {
		return []Row{}, nil
	}
	var rows []Row
	for i, line := range strings.Split(string(out), "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		active := "stopped"
		if fields[0] != "-" {
			active = "running"
		}
		rows = append(rows, Row{
			"ref":         ref("service", fields[2], fields[2]),
			"unit":        fields[2],
			"name":        fields[2],
			"load":        "loaded",
			"active":      active,
			"sub":         fields[1],
			"description": fields[2],
		})
	}
	return rows, nil
}

func windowsServices(ctx context.Context) ([]Row, error) {
	path, err := exec.LookPath("sc.exe")
	if err != nil {
		return []Row{}, nil
	}
	out, err := exec.CommandContext(ctx, path, "query", "state=", "all").Output()
	if err != nil {
		return []Row{}, nil
	}
	var rows []Row
	var cur Row
	flush := func() {
		if len(cur) > 0 {
			rows = append(rows, cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if name, ok := strings.CutPrefix(line, "SERVICE_NAME:"); ok {
			flush()
			name = strings.TrimSpace(name)
			cur = Row{"ref": ref("service", name, name), "unit": name, "name": name, "load": "loaded"}
			continue
		}
		if cur == nil {
			continue
		}
		if display, ok := strings.CutPrefix(line, "DISPLAY_NAME:"); ok {
			cur["description"] = strings.TrimSpace(display)
			continue
		}
		if state, ok := strings.CutPrefix(line, "STATE"); ok {
			_, state, _ = strings.Cut(state, ":")
			fields := strings.Fields(state)
			if len(fields) > 1 {
				cur["active"] = strings.ToLower(fields[1])
				cur["sub"] = strings.Join(fields[1:], " ")
			}
		}
	}
	flush()
	return rows, nil
}

func (l *Local) Disks(ctx context.Context) ([]Row, error) {
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return []Row{}, nil
	}
	rows := make([]Row, 0, len(parts))
	for _, part := range parts {
		row := Row{
			"ref":        ref("disk", part.Mountpoint, part.Mountpoint),
			"device":     part.Device,
			"mountpoint": part.Mountpoint,
			"fstype":     part.Fstype,
			"opts":       strings.Join(part.Opts, ","),
		}
		if usage, err := disk.UsageWithContext(ctx, part.Mountpoint); err == nil {
			row["total"] = usage.Total
			row["used"] = usage.Used
			row["free"] = usage.Free
			row["usedPct"] = usage.UsedPercent
			row["inodesTotal"] = usage.InodesTotal
			row["inodesUsed"] = usage.InodesUsed
			row["inodesUsedPct"] = usage.InodesUsedPercent
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (l *Local) DiskIO(ctx context.Context) ([]Row, error) {
	ios, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return []Row{}, nil
	}
	rows := make([]Row, 0, len(ios))
	for name, io := range ios {
		rows = append(rows, Row{
			"ref":        ref("disk_io", name, name),
			"name":       name,
			"readCount":  io.ReadCount,
			"writeCount": io.WriteCount,
			"readBytes":  io.ReadBytes,
			"writeBytes": io.WriteBytes,
			"readTime":   io.ReadTime,
			"writeTime":  io.WriteTime,
			"ioTime":     io.IoTime,
			"serial":     io.SerialNumber,
			"label":      io.Label,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return fmt.Sprint(rows[i]["name"]) < fmt.Sprint(rows[j]["name"]) })
	return rows, nil
}

func (l *Local) Networks(ctx context.Context) ([]Row, error) {
	ifaces, err := gnet.InterfacesWithContext(ctx)
	if err != nil {
		return []Row{}, nil
	}
	counters, _ := gnet.IOCountersWithContext(ctx, true)
	byName := map[string]gnet.IOCountersStat{}
	for _, c := range counters {
		byName[c.Name] = c
	}
	rows := make([]Row, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs := make([]string, 0, len(iface.Addrs))
		for _, addr := range iface.Addrs {
			addrs = append(addrs, addr.Addr)
		}
		io := byName[iface.Name]
		rows = append(rows, Row{
			"ref":         ref("network", iface.Name, iface.Name),
			"name":        iface.Name,
			"mtu":         iface.MTU,
			"hardware":    iface.HardwareAddr,
			"flags":       strings.Join(iface.Flags, ","),
			"addresses":   strings.Join(addrs, ", "),
			"bytesRecv":   io.BytesRecv,
			"bytesSent":   io.BytesSent,
			"packetsRecv": io.PacketsRecv,
			"packetsSent": io.PacketsSent,
			"errorsIn":    io.Errin,
			"errorsOut":   io.Errout,
			"dropsIn":     io.Dropin,
			"dropsOut":    io.Dropout,
		})
	}
	return rows, nil
}

func (l *Local) Connections(ctx context.Context) ([]Row, error) {
	conns, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return []Row{}, nil
	}
	rows := make([]Row, 0, len(conns))
	for _, c := range conns {
		uid := fmt.Sprintf("%d:%s:%d:%s:%d:%s", c.Pid, c.Laddr.IP, c.Laddr.Port, c.Raddr.IP, c.Raddr.Port, c.Status)
		rows = append(rows, Row{
			"ref":        ref("connection", uid, uid),
			"pid":        c.Pid,
			"family":     c.Family,
			"type":       c.Type,
			"localAddr":  endpoint(c.Laddr.IP, c.Laddr.Port),
			"remoteAddr": endpoint(c.Raddr.IP, c.Raddr.Port),
			"status":     strings.ToLower(c.Status),
			"uids":       joinInts(c.Uids),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return fmt.Sprint(rows[i]["localAddr"]) < fmt.Sprint(rows[j]["localAddr"])
	})
	if l.opts.ConnectionLimit > 0 && len(rows) > l.opts.ConnectionLimit {
		rows = rows[:l.opts.ConnectionLimit]
	}
	return rows, nil
}

func (l *Local) Users(ctx context.Context) ([]Row, error) {
	users, err := host.UsersWithContext(ctx)
	if err != nil {
		return []Row{}, nil
	}
	rows := make([]Row, 0, len(users))
	for _, u := range users {
		uid := strings.Join([]string{u.User, u.Terminal, u.Host, strconv.Itoa(u.Started)}, "|")
		rows = append(rows, Row{
			"ref":       ref("user_session", uid, u.User),
			"user":      u.User,
			"terminal":  u.Terminal,
			"host":      u.Host,
			"startedAt": unixSeconds(uint64(u.Started)),
		})
	}
	return rows, nil
}

func (l *Local) Sensors(ctx context.Context) ([]Row, error) {
	temps, err := sensors.TemperaturesWithContext(ctx)
	if err != nil {
		return []Row{}, nil
	}
	rows := make([]Row, 0, len(temps))
	for _, t := range temps {
		uid := t.SensorKey
		if uid == "" {
			uid = strconv.FormatFloat(t.Temperature, 'f', 2, 64)
		}
		rows = append(rows, Row{
			"ref":         ref("sensor", uid, uid),
			"sensor":      t.SensorKey,
			"temperature": t.Temperature,
			"high":        t.High,
			"critical":    t.Critical,
		})
	}
	return rows, nil
}

func (l *Local) CPUInfo(ctx context.Context) ([]Row, error) {
	infos, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return []Row{}, nil
	}
	rows := make([]Row, 0, len(infos))
	for _, info := range infos {
		uid := strconv.Itoa(int(info.CPU))
		rows = append(rows, Row{
			"ref":       ref("cpu", uid, "CPU "+uid),
			"cpu":       info.CPU,
			"vendor":    info.VendorID,
			"family":    info.Family,
			"model":     info.Model,
			"modelName": info.ModelName,
			"stepping":  info.Stepping,
			"mhz":       info.Mhz,
			"cacheSize": info.CacheSize,
			"cores":     info.Cores,
			"flags":     strings.Join(info.Flags, ", "),
		})
	}
	return rows, nil
}

type Remote struct {
	base   string
	client *http.Client
}

func NewRemote(base string, client *http.Client) *Remote {
	if client == nil {
		client = http.DefaultClient
	}
	return &Remote{base: strings.TrimRight(base, "/"), client: client}
}

func (r *Remote) Overview(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, r.get(ctx, "/overview", &out)
}

func (r *Remote) Metrics(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, r.get(ctx, "/metrics", &out)
}

func (r *Remote) Processes(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/processes", &out)
}

func (r *Remote) Services(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/services", &out)
}

func (r *Remote) Disks(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/disks", &out)
}

func (r *Remote) DiskIO(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/disk-io", &out)
}

func (r *Remote) Networks(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/networks", &out)
}

func (r *Remote) Connections(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/connections", &out)
}

func (r *Remote) Users(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/users", &out)
}

func (r *Remote) Sensors(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/sensors", &out)
}

func (r *Remote) CPUInfo(ctx context.Context) ([]Row, error) {
	var out []Row
	return out, r.get(ctx, "/cpu", &out)
}

func (r *Remote) get(ctx context.Context, path string, dst any) error {
	u, err := url.JoinPath(r.base, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("host monitor agent returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func Handler(backend Backend) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/overview", jsonEndpoint(backend.Overview))
	mux.HandleFunc("/metrics", jsonEndpoint(backend.Metrics))
	mux.HandleFunc("/processes", jsonEndpoint(backend.Processes))
	mux.HandleFunc("/services", jsonEndpoint(backend.Services))
	mux.HandleFunc("/disks", jsonEndpoint(backend.Disks))
	mux.HandleFunc("/disk-io", jsonEndpoint(backend.DiskIO))
	mux.HandleFunc("/networks", jsonEndpoint(backend.Networks))
	mux.HandleFunc("/connections", jsonEndpoint(backend.Connections))
	mux.HandleFunc("/users", jsonEndpoint(backend.Users))
	mux.HandleFunc("/sensors", jsonEndpoint(backend.Sensors))
	mux.HandleFunc("/cpu", jsonEndpoint(backend.CPUInfo))
	return mux
}

func jsonEndpoint[T any](fn func(context.Context) (T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		out, err := fn(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

func unixSeconds(sec uint64) string {
	if sec == 0 {
		return ""
	}
	return time.Unix(int64(sec), 0).UTC().Format(time.RFC3339)
}

func unixMillis(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func numeric(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	case json.Number:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	default:
		return 0
	}
}

func ref(kind, uid, name string) map[string]string {
	if name == "" {
		name = uid
	}
	return map[string]string{"kind": kind, "uid": uid, "name": name}
}

func endpoint(ip string, port uint32) string {
	if ip == "" && port == 0 {
		return ""
	}
	if ip == "" {
		return strconv.Itoa(int(port))
	}
	if port == 0 {
		return ip
	}
	return ip + ":" + strconv.Itoa(int(port))
}

func joinInts(values []int32) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(int(value)))
	}
	return strings.Join(parts, ",")
}
