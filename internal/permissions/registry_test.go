package permissions

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterProtocolPermission(t *testing.T) {
	err := RegisterProtocolPermission("testdriver", "connect", &Permission{
		DisplayName:  "Test Connect",
		Description:  "Connect to test driver",
		DefaultScope: "resource",
		DependsOn:    []string{"connection.launch"},
	})
	require.NoError(t, err)

	def, ok := Get("protocol:testdriver.connect")
	require.True(t, ok)
	require.Equal(t, "protocols.testdriver", def.Module)
	require.Equal(t, "protocol:testdriver", def.Category)
	require.Equal(t, "Test Connect", def.DisplayName)
	require.Contains(t, def.Metadata, "driver")
	require.Equal(t, "testdriver", def.Metadata["driver"])
	require.Contains(t, def.Metadata, "action")
	require.Equal(t, "connect", def.Metadata["action"])

	err = RegisterProtocolPermission("testdriver", "connect", &Permission{
		DisplayName: "Duplicate",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, errDuplicateID))
}
