package provider

import "fmt"

// Model identifies the DeepSeek model used for a session.
type Model string

const (
	ModelV4Flash Model = "deepseek-v4-flash"
	ModelV4Pro   Model = "deepseek-v4-pro"
)

// ReasoningEffort is the thinking budget when thinking is enabled.
type ReasoningEffort string

const (
	ReasoningLow  ReasoningEffort = "low"
	ReasoningHigh ReasoningEffort = "high"
	ReasoningMax  ReasoningEffort = "max"
)

// GenerationSettings are the per-session model and thinking parameters.
// They are immutable after approval and live only in session memory.
type GenerationSettings struct {
	Model           Model
	ThinkingEnabled bool
	// ReasoningEffort is only meaningful when ThinkingEnabled is true; it is
	// normalized to "" when thinking is disabled.
	ReasoningEffort ReasoningEffort
}

// DefaultGenerationSettings returns the canonical product defaults:
// deepseek-v4-flash with thinking enabled at high effort.
func DefaultGenerationSettings() GenerationSettings {
	return GenerationSettings{Model: ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: ReasoningHigh}
}

// Valid reports whether the settings are in normalized internal form.
func (s GenerationSettings) Valid() bool {
	switch s.Model {
	case ModelV4Flash, ModelV4Pro:
	default:
		return false
	}
	if s.ThinkingEnabled {
		switch s.ReasoningEffort {
		case ReasoningLow, ReasoningHigh, ReasoningMax:
			return true
		default:
			return false
		}
	}
	return s.ReasoningEffort == ""
}

// ParseSettings validates the raw approve fields. Empty fields fall back to
// defaults (defaultModel is the gateway's configured default, typically from
// DS_MODEL). Explicit invalid values produce an error so the approve request
// is rejected without consuming the pairing code.
func ParseSettings(defaultModel Model, modelRaw string, thinking *bool, effortRaw string) (GenerationSettings, error) {
	s := DefaultGenerationSettings()
	if defaultModel != "" {
		switch defaultModel {
		case ModelV4Flash, ModelV4Pro:
			s.Model = defaultModel
		default:
			// Defensive: an invalid configured default (e.g. a legacy alias
			// in DS_MODEL) must not break approval.
			s.Model = ModelV4Flash
		}
	}
	if modelRaw != "" {
		m := Model(modelRaw)
		switch m {
		case ModelV4Flash, ModelV4Pro:
			s.Model = m
		default:
			return GenerationSettings{}, fmt.Errorf("unsupported model %q", modelRaw)
		}
	}
	if thinking != nil {
		s.ThinkingEnabled = *thinking
	}
	if s.ThinkingEnabled {
		if effortRaw != "" {
			e := ReasoningEffort(effortRaw)
			switch e {
			case ReasoningLow, ReasoningHigh, ReasoningMax:
				s.ReasoningEffort = e
			default:
				return GenerationSettings{}, fmt.Errorf("unsupported reasoning effort %q", effortRaw)
			}
		}
		// Empty effort with thinking enabled keeps the default (high).
	} else {
		s.ReasoningEffort = ""
	}
	return s, nil
}
