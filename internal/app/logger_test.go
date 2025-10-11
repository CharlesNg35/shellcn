package app

import "testing"

import "github.com/stretchr/testify/require"

func TestConfigureLogging(t *testing.T) {
	require.NoError(t, ConfigureLogging("debug"))
	require.NoError(t, ConfigureLogging(""))
}
