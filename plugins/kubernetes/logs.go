package kubernetes

import (
	"context"
	"errors"
	"io"

	corev1 "k8s.io/api/core/v1"

	"github.com/charlesng/shellcn/internal/plugin"
)

// LogsStream streams a pod container's logs to the client. Pod logs are a plain
// chunked GET (no upgrade), so this works over both direct and agent transport
// through the session's REST client.
func LogsStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	ns, pod := rc.Param("namespace"), rc.Param("name")
	if pod == "" {
		return errors.New("pod name is required")
	}
	opts := &corev1.PodLogOptions{
		Container:  param(rc, "container"),
		Follow:     boolParam(rc, "follow", true),
		Previous:   boolParam(rc, "previous", false),
		Timestamps: boolParam(rc, "timestamps", false),
	}
	if tail := intParam(rc, "tail"); tail > 0 {
		t := int64(tail)
		opts.TailLines = &t
	}
	stream, err := s.Clientset().CoreV1().Pods(ns).GetLogs(pod, opts).Stream(rc.Ctx)
	if err != nil {
		return apiErr(err)
	}
	defer func() { _ = stream.Close() }()

	_, err = io.Copy(client, stream)
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
