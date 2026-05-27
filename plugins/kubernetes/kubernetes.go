// Package kubernetes implements the Kubernetes protocol plugin: a Lens-grade
// operations cockpit rendered entirely from the manifest projection over the
// generic renderer. It reaches the API server with client-go either directly
// (kubeconfig) or through the L7 (http_proxy) agent, which injects the cluster's
// own ServiceAccount credentials.
package kubernetes

import (
	"context"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// In-cluster API + ServiceAccount mount paths. Kubernetes-specific, so they live
// here and are passed to the plugin-agnostic agent as opaque token/CA file paths.
const (
	inClusterAPI       = "https://kubernetes.default.svc"
	inClusterTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec // path, not a secret
	inClusterCAFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:  plugin.CurrentAPIVersion,
		Name:        "kubernetes",
		Version:     "0.1.0",
		Title:       "Kubernetes",
		Description: "Kubernetes cockpit: categorized resource navigation, live workloads, pod logs/exec/port-forward, YAML apply, and cluster/node/workload metrics — over direct (kubeconfig) or in-cluster agent transport.",
		Icon:        plugin.Icon{Type: plugin.IconSVG, Value: kubernetesIconSVG},
		Category:    plugin.CategoryOrchestration,
		Config:      configSchema(),
		Capabilities: []plugin.Capability{
			"workloads", "logs", "terminal", "metrics", "events", "yaml",
		},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent},
		Agent: &plugin.AgentProfile{
			Proxy: plugin.ProxyTarget{
				Mode:      plugin.AgentHTTP,
				Address:   inClusterAPI,
				Risk:      plugin.RiskPrivileged,
				TokenFile: inClusterTokenFile,
				CAFile:    inClusterCAFile,
			},
			Install: []plugin.InstallArtifact{{
				Label:    "Kubernetes manifest",
				Kind:     "k8s-manifest",
				Delivery: plugin.DeliveryURL,
				Template: `kubectl apply -f "{{.ArtifactURL}}"`,
				Content:  k8sManifestContent,
			}},
		},
		Layout:    plugin.LayoutSidebarTree,
		Tree:      tree(),
		Resources: resources(),
		Actions:   actions(),
		Streams:   streams(),
		Recording: podRecording(),
	}
}

func (p *Plugin) Routes() []plugin.Route { return Routes() }

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return Connect(ctx, cfg)
}

// k8sManifestContent is the agent install manifest served (URL-delivered) from a
// single-use signed ticket and applied with `kubectl apply -f <url>`. It deploys
// the agent into a per-connection namespace ({{.Slug}}) — so multiple connections
// targeting the same cluster get independent agents that never clobber each
// other — with a ServiceAccount bound to cluster-admin (the cockpit acts on the
// user's behalf — declared privileged). The cluster-scoped binding is uniquely
// named per connection. The enrollment token is minted into the Secret body at
// fetch time, never placed in a URL or path.
const k8sManifestContent = `apiVersion: v1
kind: Namespace
metadata:
  name: "{{.Slug}}"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: shellcn-agent
  namespace: "{{.Slug}}"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "shellcn-agent-{{.Slug}}"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: shellcn-agent
    namespace: "{{.Slug}}"
---
apiVersion: v1
kind: Secret
metadata:
  name: shellcn-agent
  namespace: "{{.Slug}}"
type: Opaque
stringData:
  connect-url: "{{.ConnectURL}}"
  enroll-token: "{{.Token}}"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shellcn-agent
  namespace: "{{.Slug}}"
  labels:
    app: shellcn-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app: shellcn-agent
  template:
    metadata:
      labels:
        app: shellcn-agent
    spec:
      serviceAccountName: shellcn-agent
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: agent
          image: "{{.Image}}"
          args: ["-connect", "$(SHELLCN_CONNECT_URL)", "-token", "$(SHELLCN_ENROLL_TOKEN)"{{if .Insecure}}, "-insecure"{{end}}]
          env:
            - name: SHELLCN_CONNECT_URL
              valueFrom:
                secretKeyRef:
                  name: shellcn-agent
                  key: connect-url
            - name: SHELLCN_ENROLL_TOKEN
              valueFrom:
                secretKeyRef:
                  name: shellcn-agent
                  key: enroll-token
          resources:
            requests:
              cpu: 25m
              memory: 32Mi
            limits:
              cpu: 250m
              memory: 128Mi
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
`

// kubernetesIconSVG is the Kubernetes wheel mark (sanitized before render).
const kubernetesIconSVG = `<svg width="800px" height="800px" viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg" fill="none"><path fill="#326DE6" d="M4.5 14.569c.214.278.539.431.874.431h5.251c.335 0 .66-.165.875-.434l3.258-4.178c.214-.278.288-.633.214-.978l-1.165-5.207a1.128 1.128 0 00-.606-.777l-4.714-2.31A1.062 1.062 0 008.002 1c-.168 0-.335.038-.485.115l-4.715 2.32a1.129 1.129 0 00-.605.777L1.032 9.42c-.084.345 0 .7.214.978L4.5 14.568z"/><path fill="#ffffff" fill-rule="evenodd" d="M12.741 9.128c.098.002.196.01.293.024l.058.013.031.008a.308.308 0 01.26.371.306.306 0 01-.396.223h-.004l-.003-.001-.003-.002a1.58 1.58 0 00-.03-.006l-.05-.01a2.55 2.55 0 01-.274-.106 2.867 2.867 0 00-.533-.157.242.242 0 00-.171.064 4.656 4.656 0 00-.131-.023 3.971 3.971 0 01-1.764 2.212c.015.042.032.083.051.123a.239.239 0 00-.023.18c.074.17.165.332.271.484.06.078.114.16.164.244l.028.057.012.025a.306.306 0 01-.381.44.308.308 0 01-.172-.18l-.01-.02a1.57 1.57 0 01-.028-.058 2.546 2.546 0 01-.089-.28 2.837 2.837 0 00-.21-.512.242.242 0 00-.156-.095l-.03-.053-.035-.064a3.97 3.97 0 01-2.823-.007l-.07.125a.25.25 0 00-.132.064 2.13 2.13 0 00-.237.548 2.518 2.518 0 01-.088.28 1.196 1.196 0 01-.025.05l-.013.027v.001a.306.306 0 01-.421.173.308.308 0 01-.173-.314.306.306 0 01.041-.12l.014-.03.026-.052c.05-.085.104-.166.164-.244.108-.156.2-.322.277-.496a.302.302 0 00-.028-.173l.056-.133A3.972 3.972 0 014.22 9.532l-.134.023a.34.34 0 00-.176-.062 2.871 2.871 0 00-.533.156c-.09.04-.181.075-.274.105a1.017 1.017 0 01-.05.011l-.03.007H3.02l-.002.002h-.005a.308.308 0 01-.397-.349.306.306 0 01.261-.245l.005-.001h.002l.006-.002c.024-.006.054-.014.076-.018.097-.013.195-.021.293-.023.186-.013.37-.043.549-.09a.422.422 0 00.131-.133l.128-.037a3.938 3.938 0 01.625-2.752l-.098-.087a.338.338 0 00-.062-.176 2.854 2.854 0 00-.455-.319 2.557 2.557 0 01-.254-.148l-.048-.038-.015-.013-.004-.003a.323.323 0 01-.076-.45.295.295 0 01.244-.107.365.365 0 01.213.08l.022.017c.016.013.034.026.046.037.072.067.139.139.202.213.125.137.263.262.412.372.056.03.121.036.182.018l.11.078a3.938 3.938 0 012.552-1.224l.008-.129a.332.332 0 00.099-.158 2.844 2.844 0 00-.034-.553 2.56 2.56 0 01-.042-.29v-.082-.005A.306.306 0 018 2.82a.308.308 0 01.306.337v.087a2.529 2.529 0 01-.041.29 2.85 2.85 0 00-.035.553.242.242 0 00.1.153v.007l.007.129c.967.088 1.87.522 2.54 1.223l.116-.082a.34.34 0 00.186-.02c.149-.11.287-.236.412-.373.063-.075.13-.146.202-.213l.051-.04.017-.014a.307.307 0 11.381.477l-.024.02c-.015.012-.03.025-.043.034a2.537 2.537 0 01-.254.148 2.87 2.87 0 00-.455.32.241.241 0 00-.058.172l-.05.044-.058.053c.542.806.77 1.783.637 2.745l.123.036c.031.055.077.101.133.132.179.048.363.078.548.09zM7.291 5.24c.107-.024.216-.043.326-.056l-.09 1.6-.008.004a.268.268 0 01-.293.256.27.27 0 01-.135-.05l-.002.001-1.316-.93c.419-.41.945-.696 1.518-.825zm1.618 1.75l1.308-.924a3.182 3.182 0 00-1.833-.882l.09 1.598h.002a.268.268 0 00.294.256.27.27 0 00.135-.05l.004.002zm2.248 1.656L9.609 8.2l-.002-.006a.27.27 0 01-.185-.343.27.27 0 01.08-.12L9.5 7.73l1.195-1.067c.366.594.527 1.29.46 1.983zM9.096 9.5l.618 1.49a3.148 3.148 0 001.275-1.598l-1.593-.269-.002.003a.26.26 0 00-.166.023.27.27 0 00-.13.348l-.002.003zm-.385 1.905c-.573.13-1.17.1-1.727-.088l.777-1.4h.001a.27.27 0 01.475-.001h.006l.779 1.402a3.286 3.286 0 01-.311.087zm-2.418-.422l.611-1.474-.004-.006a.268.268 0 00-.297-.37L6.6 9.13l-1.579.267a3.16 3.16 0 001.272 1.586zm-.997-4.32l1.201 1.071-.001.007a.269.269 0 01-.106.462l-.001.005-1.54.443a3.134 3.134 0 01.447-1.988zm2.95 1.154h-.492l-.307.38.11.476.443.213.442-.212.11-.476-.306-.381z" clip-rule="evenodd"/></svg>`
