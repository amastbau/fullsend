package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateFilesExist_MissingPolicyNamesTheFix pins #6834's user-facing
// half: the per-repo scaffold ships no sandbox policy and CI layers none, so
// a relative policy: path that is not committed next to the harness must
// fail naming a fix that actually works for an EXISTING harness — commit a
// policy file, or reference the fleet copy by URL — not a bare stat error,
// and not a bare "run agent new" that collides on the harness that already
// exists (see TestValidateFilesExist_MissingPolicyHintDoesNotClaimAgentNewFixesIt).
func TestValidateFilesExist_MissingPolicyNamesTheFix(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "agents", "lint-docs.md")
	if err := os.MkdirAll(filepath.Dir(agent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent, []byte("# agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &Harness{Agent: agent, Policy: filepath.Join(dir, "policies", "base.yaml")}
	err := h.ValidateFilesExist()
	if err == nil {
		t.Fatal("expected an error for a missing policy")
	}
	msg := err.Error()
	for _, want := range []string{
		"policy: stat",
		"no such file or directory",
		"commit a policy file",
		"by URL",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q lacks %q", msg, want)
		}
	}

	// Present policy: no hint, no error.
	if err := os.MkdirAll(filepath.Dir(h.Policy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.Policy, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.ValidateFilesExist(); err != nil {
		t.Fatalf("unexpected error with the policy present: %v", err)
	}
}

// TestValidateFilesExist_MissingPolicyHintDoesNotClaimAgentNewFixesIt guards
// against the failure mode a review caught in an earlier revision: telling
// an operator to "just re-run agent new" for the #6834 scenario (an
// existing, already-committed harness whose policy alone went missing) is
// wrong, because Generate() collision-checks every owned file
// (harness/agents/schema/post-script) before writing anything — re-running
// it against the SAME agent name fails on that collision (or, with --force,
// rewrites the owned files) and never reaches the missing shared policy.
// The hint must not read as a blanket "run agent new" instruction.
func TestValidateFilesExist_MissingPolicyHintDoesNotClaimAgentNewFixesIt(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "agents", "lint-docs.md")
	if err := os.MkdirAll(filepath.Dir(agent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent, []byte("# agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Harness{Agent: agent, Policy: filepath.Join(dir, "policies", "base.yaml")}
	err := h.ValidateFilesExist()
	if err == nil {
		t.Fatal("expected an error for a missing policy")
	}
	msg := err.Error()
	if strings.Contains(msg, "writes policies/base.yaml when it is absent") ||
		strings.Contains(msg, "Re-run `agent new`") ||
		strings.Contains(msg, "Re-running `agent new`") {
		t.Errorf("error %q reads as a blanket instruction to re-run agent new, which fails on the harness collision for an existing agent", msg)
	}
}
