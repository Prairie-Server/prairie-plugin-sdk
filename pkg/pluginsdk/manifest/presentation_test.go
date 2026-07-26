package manifest_test

import (
	"strings"
	"testing"

	pluginv1 "github.com/prairie-server/prairie-plugin-sdk/pkg/pluginproto/prairie/plugin/v1"
	"github.com/prairie-server/prairie-plugin-sdk/pkg/pluginsdk/manifest"
)

func TestLoadRoundTripsCompletePresentation(t *testing.T) {
	raw := []byte(`{
	  "plugin_id": "prairie.example",
	  "version": "1.0.0",
	  "prairie_api_version": "v1",
	  "presentation": {
	    "display_name": "Example Plugin",
	    "summary": "A short operator-facing summary.",
	    "description_markdown": "Longer **Markdown** description.",
	    "setup_markdown": "1. Install it.\n2. Configure it.",
	    "homepage_url": "https://example.com/plugin",
	    "source_url": "https://github.com/prairie-server/example-plugin",
	    "support_url": "https://github.com/prairie-server/example-plugin/issues",
	    "changelog_url": "https://github.com/prairie-server/example-plugin/releases",
	    "publisher_name": "Prairie",
	    "publisher_url": "https://github.com/prairie-server",
	    "license_spdx": "AGPL-3.0-or-later"
	  },
	  "capabilities": [{"type": "scheduled_task.v1", "id": "example"}]
	}`)

	loaded, err := manifest.Load(raw)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	presentation := loaded.GetPresentation()
	if presentation.GetDisplayName() != "Example Plugin" {
		t.Fatalf("display_name = %q", presentation.GetDisplayName())
	}
	if presentation.GetSourceUrl() != "https://github.com/prairie-server/example-plugin" {
		t.Fatalf("source_url = %q", presentation.GetSourceUrl())
	}
	if presentation.GetLicenseSpdx() != "AGPL-3.0-or-later" {
		t.Fatalf("license_spdx = %q", presentation.GetLicenseSpdx())
	}
}

func TestLoadAcceptsAbsentAndPartialPresentation(t *testing.T) {
	for name, raw := range map[string]string{
		"absent": `{
		  "plugin_id": "prairie.example",
		  "version": "1.0.0",
		  "capabilities": [{"type": "scheduled_task.v1", "id": "example"}]
		}`,
		"partial": `{
		  "plugin_id": "prairie.example",
		  "version": "1.0.0",
		  "presentation": {"display_name": "Example Plugin"},
		  "capabilities": [{"type": "scheduled_task.v1", "id": "example"}]
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := manifest.Load([]byte(raw)); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestValidatePresentationRejectsUnsafeOrOversizedFields(t *testing.T) {
	tests := []struct {
		name         string
		presentation *pluginv1.PluginPresentation
	}{
		{name: "relative URL", presentation: &pluginv1.PluginPresentation{SourceUrl: "/source"}},
		{name: "unsafe scheme", presentation: &pluginv1.PluginPresentation{SupportUrl: "javascript:alert(1)"}},
		{name: "URL credentials", presentation: &pluginv1.PluginPresentation{HomepageUrl: "https://user:secret@example.com"}},
		{name: "leading whitespace", presentation: &pluginv1.PluginPresentation{DisplayName: " Example Plugin"}},
		{name: "trailing whitespace", presentation: &pluginv1.PluginPresentation{Summary: "Example summary. "}},
		{name: "whitespace only", presentation: &pluginv1.PluginPresentation{PublisherName: " \t"}},
		{name: "summary too long", presentation: &pluginv1.PluginPresentation{Summary: strings.Repeat("x", 241)}},
		{name: "markdown too long", presentation: &pluginv1.PluginPresentation{SetupMarkdown: strings.Repeat("x", (32<<10)+1)}},
		{name: "control character", presentation: &pluginv1.PluginPresentation{PublisherName: "Prairie\u0001"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := manifest.Validate(&pluginv1.PluginManifest{
				PluginId:     "prairie.example",
				Version:      "1.0.0",
				Presentation: test.presentation,
			})
			if err == nil {
				t.Fatal("Validate() error = nil, want presentation validation error")
			}
		})
	}
}

func TestValidatePresentationAllowsMarkdownWhitespace(t *testing.T) {
	err := manifest.Validate(&pluginv1.PluginManifest{
		PluginId: "prairie.example",
		Version:  "1.0.0",
		Presentation: &pluginv1.PluginPresentation{
			DescriptionMarkdown: "First paragraph.\n\n- One\n- Two\n",
		},
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateCatalogPresentationRequiresCompleteMetadataAndCanonicalSource(t *testing.T) {
	complete := &pluginv1.PluginManifest{
		PluginId: "prairie.example",
		Version:  "1.0.0",
		Presentation: &pluginv1.PluginPresentation{
			DisplayName:         "Example Plugin",
			Summary:             "Example summary.",
			DescriptionMarkdown: "Example description.",
			SetupMarkdown:       "Install and configure it.",
			HomepageUrl:         "https://example.com",
			SourceUrl:           "https://github.com/prairie-server/example-plugin",
			SupportUrl:          "https://github.com/prairie-server/example-plugin/issues",
			ChangelogUrl:        "https://github.com/prairie-server/example-plugin/releases",
			PublisherName:       "Prairie",
			PublisherUrl:        "https://github.com/prairie-server",
			LicenseSpdx:         "AGPL-3.0-or-later",
		},
	}

	if err := manifest.ValidateCatalogPresentation(complete, "https://github.com/prairie-server/example-plugin"); err != nil {
		t.Fatalf("ValidateCatalogPresentation() error = %v", err)
	}

	missing := &pluginv1.PluginManifest{
		PluginId: "prairie.example",
		Version:  "1.0.0",
		Presentation: &pluginv1.PluginPresentation{
			DisplayName: "Example Plugin",
		},
	}
	if err := manifest.ValidateCatalogPresentation(missing, "https://github.com/prairie-server/example-plugin"); err == nil {
		t.Fatal("missing presentation fields were accepted")
	}

	if err := manifest.ValidateCatalogPresentation(complete, "https://github.com/prairie-server/other-plugin"); err == nil {
		t.Fatal("mismatched source_url was accepted")
	}
}
