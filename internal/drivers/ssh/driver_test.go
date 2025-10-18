package ssh_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/drivers"
	sshdriver "github.com/charlesng35/shellcn/internal/drivers/ssh"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func TestSSHDriverRegistration(t *testing.T) {
	registry := drivers.DefaultRegistry()

	drv, ok := registry.Get(sshdriver.DriverIDSSH)
	require.True(t, ok, "ssh driver should be registered")
	require.Equal(t, sshdriver.DriverIDSSH, drv.ID())
	require.Equal(t, "ssh", drv.Module())
	require.Equal(t, "SSH", drv.Name())

	caps, err := drv.Capabilities(context.Background())
	require.NoError(t, err)
	require.True(t, caps.Terminal)
	require.True(t, caps.FileTransfer)
	require.True(t, caps.SessionRecording)
	require.True(t, caps.Reconnect)
	require.True(t, caps.Extras["shareable"])
	require.True(t, caps.Extras["sftp"])
}

func TestSFTPDriverRegistration(t *testing.T) {
	registry := drivers.DefaultRegistry()

	drv, ok := registry.Get(sshdriver.DriverIDSFTP)
	require.True(t, ok, "sftp driver should be registered")
	require.Equal(t, sshdriver.DriverIDSFTP, drv.ID())
	require.Equal(t, "ssh", drv.Module())
	require.Equal(t, "SFTP", drv.Name())

	caps, err := drv.Capabilities(context.Background())
	require.NoError(t, err)
	require.False(t, caps.Terminal)
	require.True(t, caps.FileTransfer)
	require.False(t, caps.SessionRecording)
	require.True(t, caps.Extras["ssh_transport"])
}

func TestSSHCredentialTemplate(t *testing.T) {
	registry := drivers.DefaultRegistry()
	raw, ok := registry.Get(sshdriver.DriverIDSSH)
	require.True(t, ok)

	templater, ok := raw.(drivers.CredentialTemplater)
	require.True(t, ok, "ssh driver should expose credential template")

	tpl, err := templater.CredentialTemplate()
	require.NoError(t, err)
	require.Equal(t, sshdriver.DriverIDSSH, tpl.DriverID)
	require.Contains(t, tpl.CompatibleProtocols, sshdriver.DriverIDSSH)
	require.Contains(t, tpl.CompatibleProtocols, sshdriver.DriverIDSFTP)
	require.NotEmpty(t, tpl.Fields)

	fieldKeys := make(map[string]struct{}, len(tpl.Fields))
	for _, field := range tpl.Fields {
		fieldKeys[field.Key] = struct{}{}
	}
	require.Contains(t, fieldKeys, "username")
	require.Contains(t, fieldKeys, "auth_method")
}

func TestSSHPermissionsRegistered(t *testing.T) {
	require.NotNil(t, lookupPermission("protocol:ssh.connect"))
	require.NotNil(t, lookupPermission("protocol:ssh.sftp"))
	require.NotNil(t, lookupPermission("protocol:ssh.share"))
	require.NotNil(t, lookupPermission("protocol:ssh.grant_write"))
	require.NotNil(t, lookupPermission("protocol:ssh.record"))
	require.NotNil(t, lookupPermission("protocol:ssh.manage_snippets"))
}

func lookupPermission(id string) *permissions.Permission {
	perm, _ := permissions.Get(id)
	return perm
}
