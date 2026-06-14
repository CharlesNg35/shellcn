// Package dbcred contains reusable credential handling for database plugins.
package dbcred

import (
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type AuthMaterial struct {
	Username                string
	Password                string
	TLSMode                 string
	ClientCertificate       string
	UsedTLSClientCredential bool
}

func ResolvedKind(cfg plugin.ConnectConfig, field string) plugin.CredentialKind {
	return cfg.CredentialKindFor(field)
}

func ResolvedValue(cfg plugin.ConnectConfig, field, key string) string {
	return cfg.CredentialValueFor(field, key)
}

func ApplyPasswordCredential(cfg plugin.ConnectConfig, username, password string) AuthMaterial {
	username = strings.TrimSpace(username)
	if identity := cfg.CredentialValueFor(plugin.CredentialRefField, "username"); identity != "" {
		username = identity
	}
	if secret := cfg.CredentialValueFor(plugin.CredentialRefField, "password"); secret != "" {
		password = secret
	}
	return AuthMaterial{Username: username, Password: password}
}

func ApplyClientCertificateCredential(cfg plugin.ConnectConfig, field, username, tlsMode, clientCertificate string) AuthMaterial {
	username = strings.TrimSpace(username)
	tlsMode = strings.TrimSpace(tlsMode)
	if identity := ResolvedValue(cfg, field, "subject"); identity != "" {
		username = identity
	}
	certificate := ResolvedValue(cfg, field, "certificate")
	privateKey := ResolvedValue(cfg, field, "private_key")
	if certificate != "" || privateKey != "" {
		clientCertificate = certificate + "\n" + privateKey
	}
	if clientCertificate != "" && (tlsMode == "" || tlsMode == "disable") {
		tlsMode = "require"
	}
	return AuthMaterial{
		Username:                username,
		TLSMode:                 tlsMode,
		ClientCertificate:       clientCertificate,
		UsedTLSClientCredential: clientCertificate != "",
	}
}
