package cli

import (
	"strings"

	"github.com/fullsend-ai/fullsend/internal/harness"
	"github.com/fullsend-ai/fullsend/internal/resolve"
)

// isBuiltinTriageFallback limits the inference adaptation to the first-party
// triage harness fetched by the agents-repo fallback. Configured harnesses
// retain their declared providers and host files.
func isBuiltinTriageFallback(agentName string, deps []harness.Dependency) bool {
	if !strings.EqualFold(agentName, "triage") || len(deps) == 0 {
		return false
	}
	url := deps[0].URL
	return strings.HasPrefix(url, defaultAgentsRepoURLPrefix) && strings.HasSuffix(url, "/harness/triage.yaml")
}

// maybeAdaptBuiltinTriage selects the fork's OpenAI pilot only for the
// fallback harness on Codex. A source-registered harness remains authoritative.
func maybeAdaptBuiltinTriage(h *harness.Harness, result *resolve.ResolveResult, runtimeName, agentName, configuredSource string, deps []harness.Dependency) bool {
	if runtimeName != "codex" || configuredSource != "" || !isBuiltinTriageFallback(agentName, deps) {
		return false
	}
	adaptBuiltinTriageForCodex(h, result)
	return true
}

// adaptBuiltinTriageForCodex replaces only the first-party triage harness's
// Vertex wiring. The agent, scripts, forge providers and security policy are
// unchanged. OpenAI's profile and run-scoped credential are supplied by the
// runner when it sees the bare openai provider name.
func adaptBuiltinTriageForCodex(h *harness.Harness, result *resolve.ResolveResult) {
	h.HostFiles = filterTriageVertexHostFiles(h.HostFiles)
	h.Providers = filterTriageVertexProviderNames(h.Providers)
	result.Providers = filterTriageVertexProviders(result.Providers)
	result.Profiles = filterTriageVertexProfiles(result.Profiles)
	for _, provider := range h.Providers {
		if provider == "openai" {
			return
		}
	}
	h.Providers = append(h.Providers, "openai")
}

func filterTriageVertexHostFiles(files []harness.HostFile) []harness.HostFile {
	kept := files[:0]
	for _, file := range files {
		switch file.Dest {
		case "/sandbox/workspace/.env.d/gcp-vertex.env", "/tmp/.gcp-credentials.json", "/sandbox/workspace/.gcp-oidc-token":
			continue
		}
		kept = append(kept, file)
	}
	return kept
}

func filterTriageVertexProviderNames(names []string) []string {
	kept := names[:0]
	for _, name := range names {
		if name != "vertex-ai" {
			kept = append(kept, name)
		}
	}
	return kept
}

func filterTriageVertexProviders(providers []resolve.ResolvedProvider) []resolve.ResolvedProvider {
	kept := providers[:0]
	for _, provider := range providers {
		if provider.Def.Name != "vertex-ai" {
			kept = append(kept, provider)
		}
	}
	return kept
}

func filterTriageVertexProfiles(profiles []resolve.ResolvedProfile) []resolve.ResolvedProfile {
	kept := profiles[:0]
	for _, profile := range profiles {
		if profile.ID != "fullsend-vertex-ai" {
			kept = append(kept, profile)
		}
	}
	return kept
}
