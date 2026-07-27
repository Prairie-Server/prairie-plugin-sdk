package convert_test

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/prairie-server/prairie-plugin-sdk/pkg/pluginproto/prairie/plugin/v1"
	"github.com/prairie-server/prairie-plugin-sdk/pkg/pluginsdk/convert"
)

func TestDecodeCapability_FullMetadataAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil metadata", func(t *testing.T) {
		got, err := convert.DecodeCapability(convert.CapabilityRecord{Type: "t", ID: "id"})
		if err != nil {
			t.Fatalf("DecodeCapability: %v", err)
		}
		if got.GetType() != "t" || got.GetId() != "id" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("auth modes icon and config schema maps", func(t *testing.T) {
		got, err := convert.DecodeCapability(convert.CapabilityRecord{
			Type: "auth_provider.v1",
			ID:   "oauth",
			Metadata: map[string]any{
				"display_name": "OAuth",
				"description":  "desc",
				"auth_modes":   []string{"oauth2", "device"},
				"icon_url":     "https://example.test/icon.png",
				"config_schema": []map[string]any{
					{
						"key":         "connection",
						"title":       "Connection",
						"description": "creds",
						"json_schema": `{"type":"object"}`,
						"required":    true,
					},
				},
				"metadata": map[string]any{"k": "v"},
			},
		})
		if err != nil {
			t.Fatalf("DecodeCapability: %v", err)
		}
		if got.GetIconUrl() == "" || len(got.GetAuthModes()) != 2 {
			t.Fatalf("unexpected: %+v", got)
		}
		if len(got.GetConfigSchema()) != 1 || got.GetConfigSchema()[0].GetKey() != "connection" {
			t.Fatalf("config schema: %+v", got.GetConfigSchema())
		}
		if got.GetMetadata().AsMap()["k"] != "v" {
			t.Fatalf("metadata: %+v", got.GetMetadata())
		}
	})

	t.Run("subscriptions any slice", func(t *testing.T) {
		got, err := convert.DecodeCapability(convert.CapabilityRecord{
			Type: "event_consumer.v1",
			ID:   "c",
			Metadata: map[string]any{
				"subscriptions": []any{"a", "b"},
			},
		})
		if err != nil {
			t.Fatalf("DecodeCapability: %v", err)
		}
		if len(got.GetSubscriptions()) != 2 {
			t.Fatalf("subscriptions = %v", got.GetSubscriptions())
		}
	})

	t.Run("bad subscriptions type", func(t *testing.T) {
		if _, err := convert.DecodeCapability(convert.CapabilityRecord{
			Metadata: map[string]any{"subscriptions": "nope"},
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad subscription element", func(t *testing.T) {
		if _, err := convert.DecodeCapability(convert.CapabilityRecord{
			Metadata: map[string]any{"subscriptions": []any{1}},
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad auth_modes type", func(t *testing.T) {
		if _, err := convert.DecodeCapability(convert.CapabilityRecord{
			Metadata: map[string]any{"auth_modes": 42},
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad config_schema type", func(t *testing.T) {
		if _, err := convert.DecodeCapability(convert.CapabilityRecord{
			Metadata: map[string]any{"config_schema": "nope"},
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad watch sync provider", func(t *testing.T) {
		if _, err := convert.DecodeCapability(convert.CapabilityRecord{
			Metadata: map[string]any{
				"watch_sync_provider": map[string]any{
					"max_batch_size": "not-a-number",
				},
			},
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("metadata that cannot become struct", func(t *testing.T) {
		if _, err := convert.DecodeCapability(convert.CapabilityRecord{
			Metadata: map[string]any{
				"metadata": map[string]any{"bad": make(chan int)},
			},
		}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCapabilityRecordsFromManifest_ErrorsAndExtras(t *testing.T) {
	t.Parallel()

	if _, err := convert.CapabilityRecordsFromManifest(&pluginv1.PluginManifest{
		Capabilities: []*pluginv1.CapabilityDescriptor{nil},
	}); err == nil {
		t.Fatal("expected nil descriptor error")
	}

	meta, err := structpb.NewStruct(map[string]any{"x": float64(1)})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}
	records, err := convert.CapabilityRecordsFromManifest(&pluginv1.PluginManifest{
		Capabilities: []*pluginv1.CapabilityDescriptor{
			{
				Type:          "auth_provider.v1",
				Id:            "a",
				DisplayName:   "Auth",
				Description:   "d",
				AuthModes:     []string{"oauth2"},
				IconUrl:       "https://example.test/i.png",
				Subscriptions: []string{"evt"},
				Metadata:      meta,
				ConfigSchema: []*pluginv1.ConfigSchema{
					{Key: "k", Title: "T", Description: "D", JsonSchema: `{}`, Required: true},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CapabilityRecordsFromManifest: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len=%d", len(records))
	}
	if records[0].Metadata["icon_url"] != "https://example.test/i.png" {
		t.Fatalf("metadata=%v", records[0].Metadata)
	}
	if _, ok := records[0].Metadata["auth_modes"]; !ok {
		t.Fatalf("missing auth_modes: %v", records[0].Metadata)
	}
}
