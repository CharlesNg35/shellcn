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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng35/shellcn/sdk/plugin"
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
	if err := validateNamespace(ns); err != nil {
		return err
	}
	if err := validateName(pod); err != nil {
		return err
	}
	opts := podLogOptions(rc)
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
				_ = copyPrefixedLog(rc.Ctx, write, stream, podName, containerName)
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

func copyPrefixedLog(ctx context.Context, write func([]byte) (int, error), src io.Reader, podName, containerName string) error {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	prefix := "[" + podName + "/" + containerName + "] "
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
