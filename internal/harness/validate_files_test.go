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
// fail with the two ways to get one, not a bare stat error the author has
// to reverse-engineer.
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
		"fullsend agent new",
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
