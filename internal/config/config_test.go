package config

import "testing"

func TestGetEnvRemovesWindowsLineEnding(t *testing.T) {
	t.Setenv("DEFTA_TEST_CRLF", "8080\r")
	if value := getEnv("DEFTA_TEST_CRLF", "fallback"); value != "8080" {
		t.Fatalf("expected sanitized port, got %q", value)
	}
}

func TestGetEnvPreservesMeaningfulSpaces(t *testing.T) {
	t.Setenv("DEFTA_TEST_SPACES", "value with spaces")
	if value := getEnv("DEFTA_TEST_SPACES", "fallback"); value != "value with spaces" {
		t.Fatalf("expected spaces to be preserved, got %q", value)
	}
}
