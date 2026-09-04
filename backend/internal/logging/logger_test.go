package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretRedaction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bearer Token",
			input:    "Authorization: Bearer secret_token_12345",
			expected: "Authorization: Bearer [REDACTED_TOKEN]",
		},
		{
			name:     "Key-Value Password",
			input:    "password=supersecret",
			expected: "password=[REDACTED]",
		},
		{
			name:     "Private Key",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
			expected: "[REDACTED_PRIVATE_KEY]",
		},
		{
			name:     "Normal Log Message",
			input:    "Starting Mystic Hypervisor daemon on port 8443",
			expected: "Starting Mystic Hypervisor daemon on port 8443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStructuredLoggerAttrsRedaction(t *testing.T) {
	var buf bytes.Buffer
	logger := InitLogger(&buf, slog.LevelInfo)

	logger.Info("User login attempt",
		slog.String("username", "admin"),
		slog.String("password", "mysecretpassword"),
		slog.String("token", "xyz987654321"),
	)

	logOutput := buf.String()

	if strings.Contains(logOutput, "mysecretpassword") {
		t.Errorf("log output contained raw password! output: %s", logOutput)
	}

	if strings.Contains(logOutput, "xyz987654321") {
		t.Errorf("log output contained raw token! output: %s", logOutput)
	}

	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in log output, got: %s", logOutput)
	}
}
