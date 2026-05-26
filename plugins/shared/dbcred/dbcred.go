// Package dbcred contains reusable credential handling for database plugins.
package dbcred

import (
	"strings"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
)

type AuthMaterial struct {
	Username                string
	Password                string
	TLSMode                 string
	ClientCertificate       string
	UsedTLSClientCredential bool
}

func ResolvedSecret(cfg plugin.ConnectConfig, field string) string {
	return cfg.String(resolvedKey(field, "secret"))
}

func ResolvedIdentity(cfg plugin.ConnectConfig, field string) string {
	return strings.TrimSpace(cfg.String(resolvedKey(field, "identity")))
}

func ApplyPasswordCredential(cfg plugin.ConnectConfig, username, password string) AuthMaterial {
	username = strings.TrimSpace(username)
	if identity := strings.TrimSpace(cfg.String(service.CredentialIdentity)); identity != "" {
		username = identity
	}
	if secret := cfg.String(service.CredentialSecret); secret != "" {
		password = secret
	}
	return AuthMaterial{Username: username, Password: password}
}

func ApplyClientCertificateCredential(cfg plugin.ConnectConfig, field, username, tlsMode, clientCertificate string) AuthMaterial {
	username = strings.TrimSpace(username)
	tlsMode = strings.TrimSpace(tlsMode)
	if identity := ResolvedIdentity(cfg, field); identity != "" {
		username = identity
	}
	if secret := ResolvedSecret(cfg, field); secret != "" {
		clientCertificate = secret
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

func resolvedKey(field, suffix string) string {
	if field == service.CredentialField {
		switch suffix {
		case "secret":
			return service.CredentialSecret
		case "identity":
			return service.CredentialIdentity
		case "kind":
			return service.CredentialKind
		}
	}
	return "_" + field + "_" + suffix
}
