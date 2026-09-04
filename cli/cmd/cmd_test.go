package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	err := ExecuteWriter([]string{"version"}, &buf)
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "mysticctl version") {
		t.Errorf("expected version output, got %q", out)
	}
}

func TestCLIStatusCommand(t *testing.T) {
	var buf bytes.Buffer
	err := ExecuteWriter([]string{"status"}, &buf)
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Control Plane Foundation") {
		t.Errorf("expected status output, got %q", out)
	}
}

func TestCLIUnimplementedCommands(t *testing.T) {
	commands := []string{"host", "vm", "container", "network", "storage", "logs", "update", "rollback", "uninstall"}

	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			var buf bytes.Buffer
			err := ExecuteWriter([]string{command}, &buf)
			if err != nil {
				t.Fatalf("%s command failed: %v", command, err)
			}
			out := buf.String()
			if !strings.Contains(out, "[NOT IMPLEMENTED]") {
				t.Errorf("expected [NOT IMPLEMENTED] in output for command %s, got %q", command, out)
			}
		})
	}
}
