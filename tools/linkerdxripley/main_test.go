package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestCLIIntegration(t *testing.T) {
	// Sample Linkerd JSONL data
	linkerdData := `{"client.addr":"192.168.1.100:12345","client.id":"test-service.default.serviceaccount.identity.linkerd.cluster.local","host":"api-service.test.svc.cluster.local","method":"GET","processing_ns":"50000","request_bytes":"0","status":200,"timestamp":"2025-09-03T15:30:32.928995068Z","total_ns":"2500000","trace_id":"abc123","uri":"http://api-service.test.svc.cluster.local/api/v1/data?id=12345","user_agent":"TestClient/1.0","version":"HTTP/2.0"}
{"client.addr":"192.168.1.101:54321","client.id":"another-service.default.serviceaccount.identity.linkerd.cluster.local","host":"api-service.test.svc.cluster.local","method":"POST","processing_ns":"75000","request_bytes":"256","status":201,"timestamp":"2025-09-03T15:30:33.123456789Z","total_ns":"3000000","trace_id":"def456","uri":"http://api-service.test.svc.cluster.local/api/v1/create","user_agent":"TestClient/2.0","version":"HTTP/2.0"}`

	tests := []struct {
		name     string
		args     []string
		input    string
		wantErr  bool
		validate func(t *testing.T, output string)
	}{
		{
			name:    "basic conversion without host change",
			args:    []string{},
			input:   strings.Split(linkerdData, "\n")[0],
			wantErr: false,
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
					t.Fatalf("failed to parse output JSON: %v", err)
				}

				if result["method"] != "GET" {
					t.Errorf("expected method GET, got %v", result["method"])
				}
				if result["url"] != "http://api-service.test.svc.cluster.local/api/v1/data?id=12345" {
					t.Errorf("unexpected URL: %v", result["url"])
				}
				if result["timestamp"] != "2025-09-03T15:30:32.928995068Z" {
					t.Errorf("unexpected timestamp: %v", result["timestamp"])
				}
			},
		},
		{
			name:    "conversion with host change",
			args:    []string{"-host", "localhost:8080"},
			input:   strings.Split(linkerdData, "\n")[0],
			wantErr: false,
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
					t.Fatalf("failed to parse output JSON: %v", err)
				}

				if result["url"] != "http://localhost:8080/api/v1/data?id=12345" {
					t.Errorf("expected host change to localhost:8080, got %v", result["url"])
				}
			},
		},
		{
			name:    "multiple lines processing",
			args:    []string{"-host", "staging.test.com:9000"},
			input:   linkerdData,
			wantErr: false,
			validate: func(t *testing.T, output string) {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				if len(lines) != 2 {
					t.Fatalf("expected 2 output lines, got %d", len(lines))
				}

				for i, line := range lines {
					var result map[string]interface{}
					if err := json.Unmarshal([]byte(line), &result); err != nil {
						t.Fatalf("failed to parse output JSON line %d: %v", i+1, err)
					}

					url, ok := result["url"].(string)
					if !ok {
						t.Fatalf("URL is not a string in line %d", i+1)
					}

					if !strings.Contains(url, "staging.test.com:9000") {
						t.Errorf("expected host change to staging.test.com:9000 in line %d, got %s", i+1, url)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the command
			args := append([]string{"run", "main.go"}, tt.args...)
			cmd := exec.Command("go", args...)
			cmd.Dir = "."

			// Provide input via stdin
			cmd.Stdin = strings.NewReader(tt.input)

			// Capture output
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Run the command
			err := cmd.Run()

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but command succeeded")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v\nstderr: %s", err, stderr.String())
				return
			}

			// Validate output
			output := stdout.String()
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestPipelineIntegration(t *testing.T) {
	// Test the full pipeline: cat file | linkerdxripley | ripley
	linkerdData := `{"client.addr":"192.168.1.100:12345","client.id":"test-service.default.serviceaccount.identity.linkerd.cluster.local","host":"api-service.test.svc.cluster.local","method":"GET","processing_ns":"50000","request_bytes":"0","status":200,"timestamp":"2025-09-03T15:30:32.928995068Z","total_ns":"2500000","trace_id":"abc123","uri":"http://api-service.test.svc.cluster.local/api/v1/data?id=12345","user_agent":"TestClient/1.0","version":"HTTP/2.0"}`

	// First, convert Linkerd to Ripley format
	cmd1 := exec.Command("go", "run", "main.go", "-host", "localhost:8080")
	cmd1.Dir = "."
	cmd1.Stdin = strings.NewReader(linkerdData)

	output1, err := cmd1.Output()
	if err != nil {
		t.Fatalf("linkerdxripley conversion failed: %v", err)
	}

	// Verify the output is valid Ripley format
	var ripleyReq map[string]interface{}
	if err := json.Unmarshal(output1, &ripleyReq); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check required Ripley fields
	requiredFields := []string{"method", "url", "timestamp"}
	for _, field := range requiredFields {
		if _, exists := ripleyReq[field]; !exists {
			t.Errorf("missing required Ripley field: %s", field)
		}
	}

	// Verify URL transformation
	if ripleyReq["url"] != "http://localhost:8080/api/v1/data?id=12345" {
		t.Errorf("unexpected URL transformation: %v", ripleyReq["url"])
	}

	t.Logf("Successfully converted Linkerd format to Ripley format:\n%s", string(output1))
}