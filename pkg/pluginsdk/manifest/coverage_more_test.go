package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/prairie-server/prairie-plugin-sdk/pkg/pluginproto/prairie/plugin/v1"
	"github.com/prairie-server/prairie-plugin-sdk/pkg/pluginsdk/capability"
	"github.com/prairie-server/prairie-plugin-sdk/pkg/pluginsdk/manifest"
)

func validManifestJSON(t *testing.T) []byte {
	t.Helper()
	return []byte(`{
	  "plugin_id": "prairie.example",
	  "version": "1.0.0",
	  "prairie_api_version": "v1",
	  "capabilities": [
	    {"type": "scheduled_task.v1", "id": "nightly", "display_name": "Nightly", "description": "runs"}
	  ]
	}`)
}

func TestMustLoadOK(t *testing.T) {
	t.Parallel()
	m := manifest.MustLoad(validManifestJSON(t))
	if m.GetPluginId() != "prairie.example" {
		t.Fatalf("MustLoad = %+v", m)
	}
}

func TestMustLoadPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = manifest.MustLoad([]byte(`{"plugin_id":"x"}`))
}

func TestLoadFromDiskErrorsAndRegister(t *testing.T) {
	t.Parallel()

	if _, err := manifest.LoadFromDisk(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing file error")
	}

	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{"plugin_id":"only"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.LoadFromDisk(badPath); err == nil {
		t.Fatal("expected validation error")
	}

	goodPath := filepath.Join(dir, "good.json")
	if err := os.WriteFile(goodPath, validManifestJSON(t), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.LoadFromDisk(goodPath)
	if err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}

	if err := manifest.RegisterHTTPRoutes(m, &pluginv1.HttpRouteDescriptor{Method: "GET", Path: "/x"}); err != nil {
		t.Fatalf("RegisterHTTPRoutes: %v", err)
	}
	if len(m.GetHttpRoutes()) != 1 {
		t.Fatalf("routes=%v", m.GetHttpRoutes())
	}
	if err := manifest.RegisterHTTPRoutes(nil); err == nil {
		t.Fatal("expected nil manifest error")
	}

	if err := manifest.RegisterAssets(m, &pluginv1.PackagedAsset{Path: "ui/index.html"}); err != nil {
		t.Fatalf("RegisterAssets: %v", err)
	}
	if len(m.GetAssets()) != 1 {
		t.Fatalf("assets=%v", m.GetAssets())
	}
	if err := manifest.RegisterAssets(nil); err == nil {
		t.Fatal("expected nil manifest error")
	}

	fsys := fstest.MapFS{"ui/index.html": &fstest.MapFile{Data: []byte("hi")}}
	asset, err := manifest.Asset("ui/index.html", fsys)
	if err != nil || asset.GetPath() != "ui/index.html" {
		t.Fatalf("Asset = (%v, %v)", asset, err)
	}
	if _, err := manifest.Asset("missing.html", fsys); err == nil {
		t.Fatal("expected missing asset error")
	}
}

func TestValidateErrorPaths(t *testing.T) {
	t.Parallel()

	if err := manifest.Validate(nil); err == nil {
		t.Fatal("nil manifest")
	}
	if err := manifest.Validate(&pluginv1.PluginManifest{}); err == nil {
		t.Fatal("missing plugin_id")
	}
	if err := manifest.Validate(&pluginv1.PluginManifest{PluginId: "x"}); err == nil {
		t.Fatal("missing version")
	}
	if err := manifest.Validate(&pluginv1.PluginManifest{
		PluginId: "x", Version: "1",
		Capabilities: []*pluginv1.CapabilityDescriptor{{Type: "", Id: "a"}},
	}); err == nil {
		t.Fatal("empty capability type")
	}
	if err := manifest.Validate(&pluginv1.PluginManifest{
		PluginId: "x", Version: "1",
		Capabilities: []*pluginv1.CapabilityDescriptor{{Type: capability.ScheduledTask, Id: ""}},
	}); err == nil {
		t.Fatal("empty capability id")
	}
	if err := manifest.Validate(&pluginv1.PluginManifest{
		PluginId: "x", Version: "1",
		Capabilities: []*pluginv1.CapabilityDescriptor{{Type: "nope.v1", Id: "a"}},
	}); err == nil {
		t.Fatal("unknown capability")
	}
	if err := manifest.Validate(&pluginv1.PluginManifest{
		PluginId: "x", Version: "1",
		Capabilities: []*pluginv1.CapabilityDescriptor{{
			Type: capability.MetadataProvider, Id: "m",
			WatchSyncProvider: &pluginv1.WatchSyncProviderDescriptor{},
		}},
	}); err == nil {
		t.Fatal("watch sync on wrong type")
	}
}

func TestValidateCatalogPresentationAndURLs(t *testing.T) {
	t.Parallel()

	base := &pluginv1.PluginManifest{PluginId: "prairie.example", Version: "1.0.0"}
	if err := manifest.ValidateCatalogPresentation(base, ""); err == nil {
		t.Fatal("expected presentation required")
	}
	if err := manifest.ValidateCatalogPresentation(&pluginv1.PluginManifest{}, ""); err == nil {
		t.Fatal("expected validate failure")
	}

	pres := &pluginv1.PluginPresentation{
		DisplayName:          " Example ",
		Summary:              "sum",
		DescriptionMarkdown:  "desc",
		SetupMarkdown:        "setup",
		HomepageUrl:          "https://example.test",
		SourceUrl:            "https://example.test/src",
		SupportUrl:           "https://example.test/support",
		ChangelogUrl:         "https://example.test/changelog",
		PublisherName:        "Pub",
		PublisherUrl:         "https://example.test/pub",
		LicenseSpdx:          "MIT",
	}
	m := &pluginv1.PluginManifest{PluginId: "prairie.example", Version: "1.0.0", Presentation: pres}
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected whitespace display_name error")
	}
	pres.DisplayName = "Example"
	if err := manifest.ValidateCatalogPresentation(m, "https://example.test/src/"); err != nil {
		t.Fatalf("ValidateCatalogPresentation: %v", err)
	}
	if err := manifest.ValidateCatalogPresentation(m, "https://other.test/src"); err == nil {
		t.Fatal("expected source_url mismatch")
	}

	pres.HomepageUrl = "ftp://example.test"
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected bad scheme")
	}
	pres.HomepageUrl = "https://user:pass@example.test"
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected credentials rejected")
	}
	pres.HomepageUrl = " https://example.test"
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected whitespace url rejected")
	}
	pres.HomepageUrl = "https://example.test/" + strings.Repeat("a", 2100)
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected long url rejected")
	}
	pres.HomepageUrl = "https://example.test"
	pres.DisplayName = strings.Repeat("x", 121)
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected long display_name rejected")
	}
	pres.DisplayName = "Example"
	pres.DescriptionMarkdown = strings.Repeat("x", (32<<10)+1)
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected long markdown rejected")
	}
	pres.DescriptionMarkdown = "ok\nline"
	pres.Summary = "bad\x00"
	if err := manifest.Validate(m); err == nil {
		t.Fatal("expected control char rejected")
	}
}

func TestValidateConfigSchemaAdminFormEdges(t *testing.T) {
	t.Parallel()

	boolDefault, _ := structpb.NewValue(true)
	badDefault, _ := structpb.NewValue("nope")
	numDefault, _ := structpb.NewValue(float64(3))
	strDefault, _ := structpb.NewValue("ok")

	cases := []struct {
		name    string
		schema  *pluginv1.ConfigSchema
		wantErr bool
	}{
		{name: "nil schema", schema: nil},
		{name: "no form", schema: &pluginv1.ConfigSchema{Key: "k", JsonSchema: `{}`}},
		{
			name: "invalid json",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT}}},
			},
			wantErr: true,
		},
		{
			name: "non-object schema",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"string"}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT}}},
			},
			wantErr: true,
		},
		{
			name: "empty field key",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{{Key: "", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT}}},
			},
			wantErr: true,
		},
		{
			name: "duplicate field",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
				}},
			},
			wantErr: true,
		},
		{
			name: "multi_select needs array",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_MULTI_SELECT, Options: []*pluginv1.AdminFormOption{{Value: "x"}}},
				}},
			},
			wantErr: true,
		},
		{
			name: "select needs options",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SELECT},
				}},
			},
			wantErr: true,
		},
		{
			name: "multi_select needs options",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"array"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_MULTI_SELECT},
				}},
			},
			wantErr: true,
		},
		{
			name: "bad bool default",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"boolean"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SWITCH, DefaultValue: badDefault},
				}},
			},
			wantErr: true,
		},
		{
			name: "ok defaults and refs",
			schema: &pluginv1.ConfigSchema{
				Key: "k",
				JsonSchema: `{
					"type":"object",
					"properties":{
						"a":{"type":"boolean"},
						"b":{"type":"number"},
						"c":{"type":"string"},
						"d":{"type":"array"}
					}
				}`,
				AdminForm: &pluginv1.AdminFormDescriptor{
					Fields: []*pluginv1.AdminFormField{
						nil,
						{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SWITCH, DefaultValue: boolDefault},
						{Key: "b", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, DefaultValue: numDefault},
						{Key: "c", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, DefaultValue: strDefault, ShowWhen: []*pluginv1.AdminFormCondition{{Field: "a"}}, ExclusiveGroupField: "b"},
						{Key: "d", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_MULTI_SELECT, DynamicOptions: true},
					},
					Sections: []*pluginv1.AdminFormSection{
						nil,
						{Key: "s", FieldKeys: []string{"a"}, ShowWhen: []*pluginv1.AdminFormCondition{{Field: "a"}}},
					},
				},
			},
		},
		{
			name: "show_when empty field",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, ShowWhen: []*pluginv1.AdminFormCondition{{Field: ""}}},
				}},
			},
			wantErr: true,
		},
		{
			name: "show_when unknown",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, ShowWhen: []*pluginv1.AdminFormCondition{{Field: "missing"}}},
				}},
			},
			wantErr: true,
		},
		{
			name: "exclusive group unknown",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{
					{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, ExclusiveGroupField: "missing"},
				}},
			},
			wantErr: true,
		},
		{
			name: "section unknown field",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{
					Fields:   []*pluginv1.AdminFormField{{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT}},
					Sections: []*pluginv1.AdminFormSection{{Key: "s", FieldKeys: []string{"missing"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "section show_when empty",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{
					Fields:   []*pluginv1.AdminFormField{{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT}},
					Sections: []*pluginv1.AdminFormSection{{Key: "s", FieldKeys: []string{"a"}, ShowWhen: []*pluginv1.AdminFormCondition{{Field: ""}}}},
				},
			},
			wantErr: true,
		},
		{
			name: "section show_when unknown",
			schema: &pluginv1.ConfigSchema{
				Key: "k", JsonSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
				AdminForm: &pluginv1.AdminFormDescriptor{
					Fields:   []*pluginv1.AdminFormField{{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT}},
					Sections: []*pluginv1.AdminFormSection{{Key: "s", FieldKeys: []string{"a"}, ShowWhen: []*pluginv1.AdminFormCondition{{Field: "missing"}}}},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := manifest.Validate(&pluginv1.PluginManifest{
				PluginId:           "prairie.example",
				Version:            "1.0.0",
				GlobalConfigSchema: []*pluginv1.ConfigSchema{tc.schema},
			})
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	// Capability-owned config schema path.
	if err := manifest.Validate(&pluginv1.PluginManifest{
		PluginId: "prairie.example",
		Version:  "1.0.0",
		Capabilities: []*pluginv1.CapabilityDescriptor{{
			Type: capability.ScheduledTask,
			Id:   "nightly",
			ConfigSchema: []*pluginv1.ConfigSchema{{
				Key: "k", JsonSchema: `{`,
				AdminForm: &pluginv1.AdminFormDescriptor{Fields: []*pluginv1.AdminFormField{{Key: "a", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT}}},
			}},
		}},
	}); err == nil {
		t.Fatal("expected capability config schema error")
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	if _, err := manifest.Load([]byte(`not-json`)); err == nil {
		t.Fatal("expected decode error")
	}
}
