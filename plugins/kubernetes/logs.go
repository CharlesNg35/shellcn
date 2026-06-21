package kubernetes

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// previousUnavailable reports the apiserver's "no previous container instance"
// rejection — a 400 when previous logs are requested but the container has not
// restarted — so the log view can show a friendly note instead of a raw error.
func previousUnavailable(err error) bool {
	return apierrors.IsBadRequest(err) && strings.Contains(err.Error(), "previous terminated container")
}

// LogsStream streams a pod container's logs to the client. Pod logs are a plain
// chunked GET (no upgrade), so this works over both direct and agent transport
// through the session's REST client.
func LogsStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	ns, pod := rc.Param("namespace"), rc.Param("name")
	if err := validateNamespace(ns); err != nil {
		return err
	}
	if err := validateName(pod); err != nil {
		return err
	}
	base := podLogOptions(rc)
	filter := strings.TrimSpace(base.Container)
	base.Container = ""
	p, err := s.Clientset().CoreV1().Pods(ns).Get(rc.Ctx, pod, metav1.GetOptions{})
	if err != nil {
		return apiErr(err)
	}
	containers := logContainers(*p, filter)
	if len(containers) == 0 {
		return nil
	}
	// One container: stream it verbatim. Multiple (the default "all" view): fan out,
	// prefixing each line with its container so they interleave readably.
	if len(containers) == 1 {
		opts := *base
		opts.Container = containers[0]
		stream, err := s.Clientset().CoreV1().Pods(ns).GetLogs(pod, &opts).Stream(rc.Ctx)
		if err != nil {
			if base.Previous && previousUnavailable(err) {
				_, werr := io.WriteString(client, "No previous logs: this container has not restarted.\n")
				return werr
			}
			return apiErr(err)
		}
		defer func() { _ = stream.Close() }()
		_, err = io.Copy(client, stream)
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	var mu sync.Mutex
	write := func(b []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return client.Write(b)
	}
	var wg sync.WaitGroup
	for _, c := range containers {
		opts := *base
		opts.Container = c
		wg.Add(1)
		go func(container string, opts corev1.PodLogOptions) {
			defer wg.Done()
			stream, err := s.Clientset().CoreV1().Pods(ns).GetLogs(pod, &opts).Stream(rc.Ctx)
			if err != nil {
				msg := apiErr(err).Error()
				if base.Previous && previousUnavailable(err) {
					msg = "no previous logs (container has not restarted)"
				}
				_, _ = write(fmt.Appendf(nil, "[%s] %s\n", container, msg))
				return
			}
			defer func() { _ = stream.Close() }()
			_ = copyPrefixedLog(rc.Ctx, write, stream, "["+container+"] ")
		}(c, opts)
	}
	wg.Wait()
	return nil
}

func WorkloadLogsStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return err
	}
	o, err := s.get(rc, k, name)
	if err != nil {
		return apiErr(err)
	}
	selector := workloadSelector(k, o.Object)
	if selector == "" {
		_, err = fmt.Fprintf(client, "No pod selector found for %s/%s.\n", k.name, name)
		return err
	}
	pods, err := s.Clientset().CoreV1().Pods(o.GetNamespace()).List(rc.Ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return apiErr(err)
	}
	if len(pods.Items) == 0 {
		_, err = fmt.Fprintf(client, "No pods found for %s/%s.\n", k.name, name)
		return err
	}

	opts := podLogOptions(rc)
	containerFilter := strings.TrimSpace(opts.Container)
	opts.Container = ""

	var mu sync.Mutex
	write := func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return client.Write(p)
	}

	var wg sync.WaitGroup
	for i := range pods.Items {
		pod := pods.Items[i]
		for _, container := range logContainers(pod, containerFilter) {
			opts := *opts
			opts.Container = container
			wg.Add(1)
			go func(podName, containerName string, opts corev1.PodLogOptions) {
				defer wg.Done()
				stream, err := s.Clientset().CoreV1().Pods(o.GetNamespace()).GetLogs(podName, &opts).Stream(rc.Ctx)
				if err != nil {
					_, _ = write([]byte(fmt.Sprintf("[%s/%s] %s\n", podName, containerName, apiErr(err))))
					return
				}
				defer func() { _ = stream.Close() }()
				_ = copyPrefixedLog(rc.Ctx, write, stream, "["+podName+"/"+containerName+"] ")
			}(pod.Name, container, opts)
		}
	}
	wg.Wait()
	return nil
}

func podLogOptions(rc *plugin.RequestContext) *corev1.PodLogOptions {
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
	return opts
}

// PodContainers lists a pod's containers (init then app) as options for a stream
// control such as the logs container picker.
func PodContainers(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ns, name := rc.Param("namespace"), rc.Param("name")
	if err := validateNamespace(ns); err != nil {
		return nil, err
	}
	if err := validateName(name); err != nil {
		return nil, err
	}
	pod, err := s.Clientset().CoreV1().Pods(ns).Get(rc.Ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	items := make([]plugin.Option, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers)+1)
	for _, c := range pod.Spec.InitContainers {
		items = append(items, plugin.Option{Label: c.Name + " (init)", Value: c.Name})
	}
	for _, c := range pod.Spec.Containers {
		items = append(items, plugin.Option{Label: c.Name, Value: c.Name})
	}
	// Offer "All containers" (empty value → stream them merged) only when there is
	// more than one to choose between.
	if len(items) > 1 {
		items = append([]plugin.Option{{Label: "All containers", Value: ""}}, items...)
	}
	return plugin.Page[plugin.Option]{Items: items, Total: ptr(len(items))}, nil
}

func logContainers(pod corev1.Pod, filter string) []string {
	if filter != "" {
		return []string{filter}
	}
	containers := make([]string, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	for _, c := range pod.Spec.InitContainers {
		containers = append(containers, c.Name)
	}
	for _, c := range pod.Spec.Containers {
		containers = append(containers, c.Name)
	}
	return containers
}

func copyPrefixedLog(ctx context.Context, write func([]byte) (int, error), src io.Reader, prefix string) error {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		if _, err := write([]byte(prefix + scanner.Text() + "\n")); err != nil {
			return err
		}
	}
	if err := scanner.Err(); errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}
