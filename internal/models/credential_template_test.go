package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCredentialTemplateBeforeSaveValidates(t *testing.T) {
	tpl := &CredentialTemplate{}
	err := tpl.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "driver_id")

	tpl.DriverID = "ssh"
	err = tpl.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "version")

	tpl.Version = "1.0.0"
	err = tpl.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "display_name")

	tpl.DisplayName = "SSH Key"
	err = tpl.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fields")

	fields, _ := json.Marshal([]map[string]any{{"name": "private_key"}})
	tpl.Fields = fields
	err = tpl.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compatible_protocols")

	protocols, _ := json.Marshal([]string{"ssh"})
	tpl.CompatibleProtocols = protocols

	require.NoError(t, tpl.BeforeSave(nil))
}
