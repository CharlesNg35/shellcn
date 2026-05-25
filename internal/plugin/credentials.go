package plugin

import "slices"

// CredentialKindInfo is the public metadata for a reusable credential kind.
// It describes how the control-plane UI should label non-secret fields and
// which protocols a scoped credential may target.
type CredentialKindInfo struct {
	Kind                CredentialKind `json:"kind"`
	Label               string         `json:"label"`
	SecretLabel         string         `json:"secretLabel"`
	SecretMultiline     bool           `json:"secretMultiline,omitempty"`
	IdentityLabel       string         `json:"identityLabel,omitempty"`
	CompatibleProtocols []string       `json:"compatibleProtocols,omitempty"`
}

var credentialKindCatalog = []CredentialKindInfo{
	{
		Kind: CredentialSSHPrivateKey, Label: "SSH private key", SecretLabel: "Private key",
		SecretMultiline: true, IdentityLabel: "Username", CompatibleProtocols: []string{"ssh", "sftp"},
	},
	{
		Kind: CredentialSSHPassword, Label: "SSH password", SecretLabel: "Password",
		IdentityLabel: "Username", CompatibleProtocols: []string{"ssh", "sftp"},
	},
	{
		Kind: CredentialDBPassword, Label: "Database password", SecretLabel: "Password",
		IdentityLabel: "Database user",
		CompatibleProtocols: []string{
			"postgresql", "postgres", "mysql", "mariadb", "mssql", "oracle", "cockroachdb",
			"clickhouse", "cassandra", "mongodb", "redis", "influxdb", "neo4j",
			"elasticsearch", "opensearch",
		},
	},
	{
		Kind: CredentialAPIToken, Label: "API token", SecretLabel: "Token",
		IdentityLabel: "Token name / subject",
		CompatibleProtocols: []string{
			"proxmox", "kubernetes", "docker", "minio", "grafana", "prometheus", "github",
			"gitlab", "gitea", "jenkins", "buildkite", "terraform-cloud", "vault", "openbao",
			"cloudflare", "digitalocean", "hetzner", "linode", "ovh", "vultr", "tailscale",
			"zerotier", "http-api", "graphql",
		},
	},
	{
		Kind: CredentialTLSClientCert, Label: "TLS client certificate", SecretLabel: "Certificate and private key",
		SecretMultiline: true,
		CompatibleProtocols: []string{
			"docker", "ftps", "webdav", "proxmox", "kubernetes", "mysql", "postgresql",
			"postgres", "mongodb", "elasticsearch", "opensearch", "http-api", "graphql", "grpc",
		},
	},
	{
		Kind: CredentialKubeconfig, Label: "Kubeconfig", SecretLabel: "Kubeconfig YAML",
		SecretMultiline: true, IdentityLabel: "Context / user", CompatibleProtocols: []string{"kubernetes", "helm", "argocd", "flux", "velero"},
	},
	{
		Kind: CredentialCloudAccessKey, Label: "Cloud access key", SecretLabel: "Secret access key",
		IdentityLabel: "Access key ID", CompatibleProtocols: []string{"aws", "gcp", "azure", "s3", "linode", "ovh", "vultr", "digitalocean", "hetzner"},
	},
	{
		Kind: CredentialServiceAccountJSON, Label: "Service account JSON", SecretLabel: "JSON key",
		SecretMultiline: true, IdentityLabel: "Service account", CompatibleProtocols: []string{"gcp", "kubernetes", "grafana"},
	},
	{
		Kind: CredentialBasicAuth, Label: "Basic auth", SecretLabel: "Password",
		IdentityLabel: "Username", CompatibleProtocols: []string{"http-api", "graphql", "webdav", "prometheus", "grafana", "kibana"},
	},
	{
		Kind: CredentialBearerToken, Label: "Bearer token", SecretLabel: "Token",
		IdentityLabel: "Token name / subject", CompatibleProtocols: []string{"http-api", "graphql", "grpc", "prometheus", "grafana", "loki", "tempo", "jaeger"},
	},
	{
		Kind: CredentialVNCPassword, Label: "VNC password", SecretLabel: "Password",
		CompatibleProtocols: []string{"vnc"},
	},
	{
		Kind: CredentialRDPPassword, Label: "RDP password", SecretLabel: "Password",
		IdentityLabel: "Username", CompatibleProtocols: []string{"rdp"},
	},
	{
		Kind: CredentialSMBPassword, Label: "SMB password", SecretLabel: "Password",
		IdentityLabel: "Username", CompatibleProtocols: []string{"smb"},
	},
	{
		Kind: CredentialSNMPCommunity, Label: "SNMP community", SecretLabel: "Community string",
		CompatibleProtocols: []string{"snmp"},
	},
	{
		Kind: CredentialSNMPv3, Label: "SNMPv3 credentials", SecretLabel: "Auth/privacy material",
		SecretMultiline: true, IdentityLabel: "Security name", CompatibleProtocols: []string{"snmp"},
	},
}

// CredentialKinds returns a stable copy of the credential-kind catalog.
func CredentialKinds() []CredentialKindInfo {
	out := make([]CredentialKindInfo, len(credentialKindCatalog))
	copy(out, credentialKindCatalog)
	return out
}

// CredentialKindLookup returns one credential kind's metadata.
func CredentialKindLookup(kind CredentialKind) (CredentialKindInfo, bool) {
	for _, info := range credentialKindCatalog {
		if info.Kind == kind {
			return info, true
		}
	}
	return CredentialKindInfo{}, false
}

// CredentialKindSupportsProtocol reports whether a credential kind may be
// explicitly scoped to protocol. An empty compatibility list means no protocol
// restriction at the kind level.
func CredentialKindSupportsProtocol(kind CredentialKind, protocol string) bool {
	info, ok := CredentialKindLookup(kind)
	if !ok {
		return false
	}
	return len(info.CompatibleProtocols) == 0 || slices.Contains(info.CompatibleProtocols, protocol)
}
