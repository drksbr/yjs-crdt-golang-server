package cmd

import (
	"io"
	"strings"
	"testing"
)

func TestRootCommandReturnsConfigError(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("YJSBRIDGE_POSTGRES_DSN", "")

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--env-file", ""})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() err = nil, want config error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("Execute() err = %v, want DATABASE_URL config error", err)
	}
}
