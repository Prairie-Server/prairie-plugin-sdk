package config_test

import (
	"testing"

	pluginv1 "github.com/prairie-server/prairie-plugin-sdk/pkg/pluginproto/prairie/plugin/v1"
	"github.com/prairie-server/prairie-plugin-sdk/pkg/pluginsdk/config"
)

func TestValidateManifestNilAndEmptySchema(t *testing.T) {
	t.Parallel()

	if err := config.ValidateManifestGlobalValue(nil, "k", nil); err == nil {
		t.Fatal("expected nil manifest error")
	}
	if err := config.ValidateManifestUserValue(nil, "k", nil); err == nil {
		t.Fatal("expected nil manifest error")
	}

	manifest := &pluginv1.PluginManifest{
		GlobalConfigSchema: []*pluginv1.ConfigSchema{
			{Key: "empty", Title: "Empty", JsonSchema: "   "},
			{Key: "bad", Title: "Bad", JsonSchema: `{`},
			nil,
			{Key: "other", Title: "Other"},
		},
		UserConfigSchema: []*pluginv1.ConfigSchema{
			{Key: "prefs", Title: "Prefs", JsonSchema: `{"type":"object"}`},
		},
	}

	if err := config.ValidateManifestGlobalValue(manifest, "empty", nil); err != nil {
		t.Fatalf("empty schema with nil value: %v", err)
	}
	if err := config.ValidateManifestGlobalValue(manifest, "bad", map[string]any{}); err == nil {
		t.Fatal("expected invalid schema JSON error")
	}
	if err := config.ValidateManifestUserValue(manifest, "prefs", map[string]any{"x": 1}); err != nil {
		t.Fatalf("user value: %v", err)
	}

	if got := config.FindSchema(manifest.GetGlobalConfigSchema(), "other"); got == nil || got.GetKey() != "other" {
		t.Fatalf("FindSchema = %+v", got)
	}
	if got := config.FindSchema(manifest.GetGlobalConfigSchema(), "missing"); got != nil {
		t.Fatalf("FindSchema(missing) = %+v", got)
	}
}
