package configui

import (
	"bytes"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	"github.com/stretchr/testify/assert"
)

func TestPrintSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	o := NewConfigOutput(buf, false)
	o.PrintSuccess("done")
	assert.Contains(t, buf.String(), "✓")
	assert.Contains(t, buf.String(), "done")
}

func TestPrintError(t *testing.T) {
	buf := &bytes.Buffer{}
	o := NewConfigOutput(buf, false)
	o.PrintError("failed", assert.AnError)
	assert.Contains(t, buf.String(), "✗")
	assert.Contains(t, buf.String(), "failed")
}

func TestPrintTable(t *testing.T) {
	buf := &bytes.Buffer{}
	o := NewConfigOutput(buf, false)
	o.PrintTable(
		[]string{"Name", "Value"},
		[][]string{
			{"KEY", "val"},
			{"LONGER_KEY", "longer_value"},
		},
	)
	output := buf.String()
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "KEY")
	assert.Contains(t, output, "LONGER_KEY")
	assert.Contains(t, output, "───")
}

func TestPrintEntryTable_masks_secrets(t *testing.T) {
	buf := &bytes.Buffer{}
	o := NewConfigOutput(buf, false)
	entries := []appconfig.ConfigEntry{
		{Key: "PUBLIC", Value: "visible", Sensitive: false},
		{Key: "SECRET", Value: "hidden", Sensitive: true},
	}
	o.PrintEntryTable(entries, false)
	output := buf.String()
	assert.Contains(t, output, "visible")
	assert.Contains(t, output, "********")
	assert.NotContains(t, output, "hidden")
}

func TestPrintEntryTable_shows_secrets(t *testing.T) {
	buf := &bytes.Buffer{}
	o := NewConfigOutput(buf, false)
	entries := []appconfig.ConfigEntry{
		{Key: "SECRET", Value: "revealed", Sensitive: true},
	}
	o.PrintEntryTable(entries, true)
	assert.Contains(t, buf.String(), "revealed")
	assert.NotContains(t, buf.String(), "********")
}

func TestPrintStoreList(t *testing.T) {
	buf := &bytes.Buffer{}
	o := NewConfigOutput(buf, false)
	stores := []appconfig.StoreInfo{
		{ResourceName: "appConfig", StoreName: "app-config", StoreKind: appconfig.StoreKindParameterStore},
		{ResourceName: "appSecrets", StoreName: "app-secrets", StoreKind: appconfig.StoreKindSecretsManager},
	}
	o.PrintStoreList(stores)
	output := buf.String()
	assert.Contains(t, output, "app-config")
	assert.Contains(t, output, "parameter-store")
	assert.Contains(t, output, "app-secrets")
	assert.Contains(t, output, "secrets-manager")
}

func TestNoAnsiInNonColorMode(t *testing.T) {
	buf := &bytes.Buffer{}
	o := NewConfigOutput(buf, false)
	o.PrintSuccess("test")
	assert.NotContains(t, buf.String(), "\033[")
}
