package handlers

import "testing"

func TestParseAuditSuccess(t *testing.T) {
	for _, value := range []string{"", "true", "false", "1", "0"} {
		if _, valid := parseAuditSuccess(value); !valid {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	if _, valid := parseAuditSuccess("yes"); valid {
		t.Fatal("expected invalid success filter")
	}
}
