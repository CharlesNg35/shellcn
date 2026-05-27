package kubernetes

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/charlesng/shellcn/internal/plugin"
)

// PortForwardStream tunnels a single pod port to the client as a raw byte
// stream. It opens a port-forward SPDY connection (over the upgrade config, so
// it works on both transports) and pipes the client stream to the pod's data
// stream — the browser side can then proxy or surface the connection.
func PortForwardStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	ns, pod := rc.Param("namespace"), rc.Param("name")
	port := intParam(rc, "port")
	if pod == "" || port <= 0 {
		return errors.New("pod name and a positive port are required")
	}

	cfg, err := s.upgradeConfig()
	if err != nil {
		return err
	}
	cfg.GroupVersion = &corev1.SchemeGroupVersion
	cfg.APIPath = "/api"
	cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	restClient, err := rest.RESTClientFor(cfg)
	if err != nil {
		return err
	}
	u := restClient.Post().Resource("pods").Namespace(ns).Name(pod).SubResource("portforward").URL()

	rt, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return err
	}
	conn, _, err := spdy.NewDialer(upgrader, &http.Client{Transport: rt}, http.MethodPost, u).
		Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		return apiErr(err)
	}
	defer func() { _ = conn.Close() }()

	headers := http.Header{}
	headers.Set(corev1.PortHeader, strconv.Itoa(port))
	headers.Set(corev1.PortForwardRequestIDHeader, "0")

	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	errStream, err := conn.CreateStream(headers)
	if err != nil {
		return apiErr(err)
	}
	go func() { _, _ = io.Copy(io.Discard, errStream) }()

	headers.Set(corev1.StreamType, corev1.StreamTypeData)
	dataStream, err := conn.CreateStream(headers)
	if err != nil {
		return apiErr(err)
	}

	done := make(chan error, 2)
	go func() { _, e := io.Copy(dataStream, client); done <- e }()
	go func() { _, e := io.Copy(client, dataStream); done <- e }()
	select {
	case <-client.Context().Done():
		return nil
	case err := <-done:
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
}
