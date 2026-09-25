package cli

import (
	"testing"

	"github.com/fullsend-ai/fullsend/internal/harness"
	"github.com/fullsend-ai/fullsend/internal/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptBuiltinTriageForCodex(t *testing.T) {
	h := &harness.Harness{
		Role:      "triage",
		Providers: []string{"github-ro"},
		HostFiles: []harness.HostFile{
			{Src: "/cache/gcp.env", Dest: "/sandbox/workspace/.env.d/gcp-vertex.env"},
			{Src: "${GOOGLE_APPLICATION_CREDENTIALS}", Dest: "/tmp/.gcp-credentials.json"},
			{Src: "${GCP_OIDC_TOKEN_FILE}", Dest: "/sandbox/workspace/.gcp-oidc-token", Optional: true},
			{Src: "/cache/triage.env", Dest: "/sandbox/workspace/.env.d/triage.env"},
		},
	}
	result := resolve.ResolveResult{
		Profiles:  []resolve.ResolvedProfile{{ID: "fullsend-vertex-ai"}, {ID: "fullsend-github-ro"}},
		Providers: []resolve.ResolvedProvider{{Def: harness.ProviderDef{Name: "vertex-ai", Type: "fullsend-vertex-ai"}}, {Def: harness.ProviderDef{Name: "github-ro", Type: "fullsend-github-ro"}}},
	}

	adaptBuiltinTriageForCodex(h, &result)

	assert.Equal(t, []string{"github-ro", "openai"}, h.Providers)
	assert.Equal(t, []harness.HostFile{{Src: "/cache/triage.env", Dest: "/sandbox/workspace/.env.d/triage.env"}}, h.HostFiles)
	assert.Equal(t, []resolve.ResolvedProfile{{ID: "fullsend-github-ro"}}, result.Profiles)
	assert.Equal(t, []resolve.ResolvedProvider{{Def: harness.ProviderDef{Name: "github-ro", Type: "fullsend-github-ro"}}}, result.Providers)
	require.NoError(t, h.ValidateRunnerEnvWith(func(string) (string, bool) { return "", false }))

	// The helper is idempotent if called by future loading paths.
	adaptBuiltinTriageForCodex(h, &result)
	assert.Equal(t, []string{"github-ro", "openai"}, h.Providers)
}

func TestIsBuiltinTriageFallback(t *testing.T) {
	assert.True(t, isBuiltinTriageFallback("triage", []harness.Dependency{{URL: defaultAgentsRepoURLPrefix + "abc/harness/triage.yaml"}}))
	assert.False(t, isBuiltinTriageFallback("triage", nil))
	assert.False(t, isBuiltinTriageFallback("code", []harness.Dependency{{URL: defaultAgentsRepoURLPrefix + "abc/harness/code.yaml"}}))
	assert.False(t, isBuiltinTriageFallback("triage", []harness.Dependency{{URL: "https://example.com/agents/abc/harness/triage.yaml"}}))
}

func TestMaybeAdaptBuiltinTriage_OnlyCodexFallback(t *testing.T) {
	deps := []harness.Dependency{{URL: defaultAgentsRepoURLPrefix + "abc/harness/triage.yaml"}}
	for _, tc := range []struct {
		name, runtime, source string
		want                  bool
	}{
		{name: "codex fallback", runtime: "codex", want: true},
		{name: "vertex fallback", runtime: "claude", want: false},
		{name: "custom source", runtime: "codex", source: "harness/triage.yaml", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &harness.Harness{Providers: []string{"vertex-ai"}, HostFiles: []harness.HostFile{{Src: "${GOOGLE_APPLICATION_CREDENTIALS}", Dest: "/tmp/.gcp-credentials.json"}}}
			result := resolve.ResolveResult{Providers: []resolve.ResolvedProvider{{Def: harness.ProviderDef{Name: "vertex-ai", Type: "fullsend-vertex-ai"}}}}
			changed := maybeAdaptBuiltinTriage(h, &result, tc.runtime, "triage", tc.source, deps)
			assert.Equal(t, tc.want, changed)
			if tc.want {
				assert.Equal(t, []string{"openai"}, h.Providers)
				require.NoError(t, h.ValidateRunnerEnvWith(func(string) (string, bool) { return "", false }))
			} else {
				assert.Equal(t, []string{"vertex-ai"}, h.Providers)
				assert.Len(t, h.HostFiles, 1)
			}
		})
	}
}
