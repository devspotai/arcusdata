package util

import (
	"os"
	"testing"
)

// GetEnvAsInt passed "0" as the fallback to GetEnv, so an unset variable
// parsed successfully as 0 and the caller's default was never reached.
func TestGetEnvAsIntUsesDefaultWhenUnset(t *testing.T) {
	const key = "ARCUS_TEST_UNSET_PROBE"
	t.Setenv(key, "placeholder") // registers cleanup, then remove it entirely
	os.Unsetenv(key)
	if got := GetEnvAsInt(key, 300); got != 300 {
		t.Fatalf("unset variable must yield the default, got %d", got)
	}
}

func TestGetEnvAsIntReadsAndFallsBack(t *testing.T) {
	t.Setenv("ARCUS_TEST_INT", "42")
	if got := GetEnvAsInt("ARCUS_TEST_INT", 7); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}

	t.Setenv("ARCUS_TEST_INT", "not-a-number")
	if got := GetEnvAsInt("ARCUS_TEST_INT", 7); got != 7 {
		t.Fatalf("unparseable value must yield the default, got %d", got)
	}

	// An explicit zero must still be honoured.
	t.Setenv("ARCUS_TEST_INT", "0")
	if got := GetEnvAsInt("ARCUS_TEST_INT", 7); got != 0 {
		t.Fatalf("explicit 0 must be preserved, got %d", got)
	}
}

// APP_ENV is concatenated into the dotenv filename, so it must not be able
// to walk out of the working directory.
func TestLoadEnvFileRejectsTraversalInAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "../../../../etc/passwd")
	err := LoadEnvFile()
	if err == nil {
		t.Fatal("expected traversal-shaped APP_ENV to be rejected")
	}
}

func TestLoadEnvFileReturnsErrorInsteadOfExiting(t *testing.T) {
	t.Setenv("APP_ENV", "definitely-not-present")
	// Previously this called log.Fatal and killed the test binary.
	if err := LoadEnvFile(); err == nil {
		t.Fatal("expected an error for a missing env file")
	}
}
