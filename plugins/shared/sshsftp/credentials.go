package sshsftp

import "github.com/charlesng35/shellcn/sdk/plugin"

const (
	CredentialSSHPrivateKey plugin.CredentialKind = "ssh_private_key"
	CredentialSSHPassword   plugin.CredentialKind = "ssh_password"
)

func CredentialKinds() []plugin.CredentialKindInfo {
	return []plugin.CredentialKindInfo{
		{
			Kind: CredentialSSHPrivateKey, Label: "SSH private key", SecretLabel: "Private key",
			SecretMultiline: true, IdentityLabel: "Username",
		},
		{
			Kind: CredentialSSHPassword, Label: "SSH password", SecretLabel: "Password",
			IdentityLabel: "Username",
		},
	}
}
