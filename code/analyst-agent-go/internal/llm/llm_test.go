package llm

import "testing"

func TestGeminiModelIsConfigurable(t *testing.T) {
	t.Setenv("GEMINI_MODEL", "")
	want := geminiAPIBase + defaultGeminiModel + ":generateContent"
	if got := geminiURL(); got != want {
		t.Errorf("default URL = %q, want %q", got, want)
	}
	t.Setenv("GEMINI_MODEL", "  gemini-next-flash ")
	if got := geminiURL(); got != geminiAPIBase+"gemini-next-flash:generateContent" {
		t.Errorf("override ignored or untrimmed: %q", got)
	}
}

func TestDefaultModelsAreNotRetiredOnes(t *testing.T) {
	// gemini-2.0-* models were shut down on 1 June 2026.
	for _, m := range []string{defaultGeminiModel, defaultOpenRouterModel} {
		if len(m) >= 3 && (contains(m, "2.0-flash") || contains(m, "1.5-")) {
			t.Errorf("default model %q is a retired model family", m)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
