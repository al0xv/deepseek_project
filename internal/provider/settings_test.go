package provider

import "testing"

func TestDefaultGenerationSettings(t *testing.T) {
	s := DefaultGenerationSettings()
	if s.Model != ModelV4Flash {
		t.Fatalf("model = %q, want deepseek-v4-flash", s.Model)
	}
	if !s.ThinkingEnabled {
		t.Fatal("thinking must be enabled by default")
	}
	if s.ReasoningEffort != ReasoningHigh {
		t.Fatalf("effort = %q, want high", s.ReasoningEffort)
	}
	if !s.Valid() {
		t.Fatalf("defaults must be valid: %+v", s)
	}
}

func TestParseSettingsAcceptsAllCombinations(t *testing.T) {
	combos := []struct {
		modelRaw string
		thinking *bool
		effort   string
	}{
		{"deepseek-v4-flash", boolPtr(false), ""},
		{"deepseek-v4-flash", boolPtr(true), "low"},
		{"deepseek-v4-flash", boolPtr(true), "high"},
		{"deepseek-v4-flash", boolPtr(true), "max"},
		{"deepseek-v4-pro", boolPtr(false), ""},
		{"deepseek-v4-pro", boolPtr(true), "low"},
		{"deepseek-v4-pro", boolPtr(true), "high"},
		{"deepseek-v4-pro", boolPtr(true), "max"},
	}
	for _, c := range combos {
		s, err := ParseSettings(ModelV4Flash, c.modelRaw, c.thinking, c.effort)
		if err != nil {
			t.Fatalf("ParseSettings(%q, thinking=%v, effort=%q): %v", c.modelRaw, c.thinking != nil && *c.thinking, c.effort, err)
		}
		if !s.Valid() {
			t.Fatalf("parsed settings invalid: %+v", s)
		}
		if c.thinking != nil && !*c.thinking && s.ReasoningEffort != "" {
			t.Fatalf("thinking off must normalize effort to empty, got %q", s.ReasoningEffort)
		}
	}
}

func TestParseSettingsRejectsInvalidValues(t *testing.T) {
	invalid := []struct {
		modelRaw string
		thinking *bool
		effort   string
	}{
		{"deepseek-chat", boolPtr(true), "high"},
		{"deepseek-reasoner", boolPtr(true), "high"},
		{"gpt-5", boolPtr(true), "high"},
		{"random", boolPtr(true), "high"},
		{"deepseek-v4-flash", boolPtr(true), "medium"},
		{"deepseek-v4-flash", boolPtr(true), "off"},
		{"deepseek-v4-flash", boolPtr(true), "random"},
	}
	for _, c := range invalid {
		if _, err := ParseSettings(ModelV4Flash, c.modelRaw, c.thinking, c.effort); err == nil {
			t.Fatalf("expected rejection for model=%q thinking=%v effort=%q", c.modelRaw, c.thinking != nil && *c.thinking, c.effort)
		}
	}
}

func TestParseSettingsEmptyFieldsUseDefaults(t *testing.T) {
	s, err := ParseSettings(ModelV4Flash, "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if s != DefaultGenerationSettings() {
		t.Fatalf("empty fields must yield defaults: %+v", s)
	}

	// Explicit effort with thinking enabled and empty model keeps the default model.
	s, err = ParseSettings(ModelV4Flash, "", boolPtr(true), "max")
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != ModelV4Flash || s.ReasoningEffort != ReasoningMax {
		t.Fatalf("parsed = %+v", s)
	}
}

func TestParseSettingsUsesDefaultModelOverride(t *testing.T) {
	// defaultModel (e.g. DS_MODEL override) applies when model is absent.
	s, err := ParseSettings(ModelV4Pro, "", boolPtr(true), "high")
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != ModelV4Pro {
		t.Fatalf("model = %q, want deepseek-v4-pro (default override)", s.Model)
	}
	// Explicit model wins over the default override.
	s, err = ParseSettings(ModelV4Pro, "deepseek-v4-flash", boolPtr(true), "high")
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != ModelV4Flash {
		t.Fatalf("model = %q, want deepseek-v4-flash (explicit)", s.Model)
	}
}

func TestGenerationSettingsValid(t *testing.T) {
	valid := []GenerationSettings{
		{Model: ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: ReasoningLow},
		{Model: ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: ReasoningHigh},
		{Model: ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: ReasoningMax},
		{Model: ModelV4Pro, ThinkingEnabled: false, ReasoningEffort: ""},
	}
	for _, s := range valid {
		if !s.Valid() {
			t.Fatalf("expected valid: %+v", s)
		}
	}
	invalid := []GenerationSettings{
		{Model: "deepseek-chat", ThinkingEnabled: true, ReasoningEffort: ReasoningHigh},
		{Model: ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: "off"},
		{Model: ModelV4Flash, ThinkingEnabled: false, ReasoningEffort: ReasoningHigh},
		{Model: ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: ""},
		{Model: "", ThinkingEnabled: true, ReasoningEffort: ReasoningHigh},
	}
	for _, s := range invalid {
		if s.Valid() {
			t.Fatalf("expected invalid: %+v", s)
		}
	}
}

func boolPtr(b bool) *bool { return &b }
