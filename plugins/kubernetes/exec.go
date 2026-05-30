package kubernetes

import (
	"encoding/json"
	"errors"
	"io"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/streaming/pkg/httpstream"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// ExecStream runs an interactive exec into a pod container, bridged to the
// terminal panel. Exec uses a SPDY/WebSocket upgrade; over agent transport that
// rides the loopback bridge (upgradeConfig), so it works on both transports.
func ExecStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	ns, pod := rc.Param("namespace"), rc.Param("name")
	if pod == "" {
		return errors.New("pod name is required")
	}
	tty := boolParam(rc, "tty", true)
	exec, err := s.podExecutor(ns, pod, &corev1.PodExecOptions{
		Container: param(rc, "container"),
		Command:   interactiveShellCommand(rc, tty),
		Stdin:     true,
		Stdout:    true,
		Stderr:    !tty,
		TTY:       tty,
	})
	if err != nil {
		return err
	}
	return streamExec(client, exec, tty, intParam(rc, "cols"), intParam(rc, "rows"))
}

// streamExec bridges an exec executor to the terminal panel: stdin (with resize
// control frames) flows in, multiplexed stdout/stderr flows out.
func streamExec(client plugin.ClientStream, exec remotecommand.Executor, tty bool, cols, rows int) error {
	stdinR, stdinW := io.Pipe()
	sizes := &termSizeQueue{ch: make(chan remotecommand.TerminalSize, 4)}
	if cols > 0 && rows > 0 {
		sizes.push(cols, rows)
	}
	go pipeTerminalInput(client, stdinW, sizes)

	out := &lockedWriter{w: client}
	opts := remotecommand.StreamOptions{Stdin: stdinR, Stdout: out, Tty: tty, TerminalSizeQueue: sizes}
	if !tty {
		opts.Stderr = out
	}
	if err := exec.StreamWithContext(client.Context(), opts); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// podExecutor builds a fallback (WebSocket → SPDY) executor against the upgrade
// config (the loopback bridge for agent transport, the kubeconfig for direct).
func (s *Session) podExecutor(ns, pod string, opts *corev1.PodExecOptions) (remotecommand.Executor, error) {
	cfg, err := s.upgradeConfig()
	if err != nil {
		return nil, err
	}
	cfg.GroupVersion = &corev1.SchemeGroupVersion
	cfg.APIPath = "/api"
	cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	client, err := rest.RESTClientFor(cfg)
	if err != nil {
		return nil, err
	}
	u := client.Post().Resource("pods").Namespace(ns).Name(pod).
		SubResource("exec").VersionedParams(opts, scheme.ParameterCodec).URL()

	spdyExec, err := remotecommand.NewSPDYExecutor(cfg, "POST", u)
	if err != nil {
		return nil, err
	}
	wsExec, err := remotecommand.NewWebSocketExecutor(cfg, "GET", u.String())
	if err != nil {
		return spdyExec, nil
	}
	return remotecommand.NewFallbackExecutor(wsExec, spdyExec, httpstream.IsUpgradeFailure)
}

// pipeTerminalInput demultiplexes the client stream: a frame led by a 0 byte is
// a control message (resize); everything else is stdin.
func pipeTerminalInput(client plugin.ClientStream, stdin *io.PipeWriter, sizes *termSizeQueue) {
	buf := make([]byte, 32<<10)
	for {
		n, err := client.Read(buf)
		if n > 0 {
			frame := buf[:n]
			if frame[0] == 0 {
				sizes.control(frame[1:])
			} else if _, werr := stdin.Write(frame); werr != nil {
				_ = stdin.CloseWithError(werr)
				return
			}
		}
		if err != nil {
			_ = stdin.CloseWithError(err)
			return
		}
	}
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

type termSizeQueue struct {
	ch chan remotecommand.TerminalSize
}

func (q *termSizeQueue) Next() *remotecommand.TerminalSize {
	s, ok := <-q.ch
	if !ok {
		return nil
	}
	return &s
}

func (q *termSizeQueue) push(cols, rows int) {
	select {
	case q.ch <- remotecommand.TerminalSize{Width: uint16(cols), Height: uint16(rows)}:
	default:
	}
}

func (q *termSizeQueue) control(frame []byte) {
	var m struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if json.Unmarshal(frame, &m) == nil && m.Type == "resize" {
		q.push(m.Cols, m.Rows)
	}
}
