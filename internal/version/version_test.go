package version

import "testing"

func TestReleaseUsesConfiguredVersion(t *testing.T) {
	t.Parallel()

	if got, want := release("sha-123456789abc", "abcdef1234567890"), "sha-123456789abc"; got != want {
		t.Fatalf("release() = %q, want %q", got, want)
	}
}

func TestPublicDefaultsToLatest(t *testing.T) {
	t.Parallel()

	if got, want := public(""), "latest"; got != want {
		t.Fatalf("public() = %q, want %q", got, want)
	}
}

func TestReleaseFallsBackToShortCommit(t *testing.T) {
	t.Parallel()

	if got, want := release("dev", "abcdef1234567890"), "sha-abcdef123456"; got != want {
		t.Fatalf("release() = %q, want %q", got, want)
	}
}

func TestCommandOutputIncludesFullCommit(t *testing.T) {
	t.Parallel()

	want := "sha-abcdef123456\ncommit abcdef1234567890"
	if got := commandOutput("sha-abcdef123456", "abcdef1234567890"); got != want {
		t.Fatalf("commandOutput() = %q, want %q", got, want)
	}
}
