package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestCLIIntegration(t *testing.T) {
	t.Run("basic conversion without host change", func(t *testing.T) {
		testBasicConversion(t)
	})

	t.Run("conversion with host change", func(t *testing.T) {
		testHostChangeConversion(t)
	})

	t.Run("multiple lines processing", func(t *testing.T) {
		testMultipleLineProcessing(t)
	})
}

func testBasicConversion(t *testing.T) {
	linkerdData := getSampleLinkerdData()[0]
	output := runCLICommand(t, []string{}, linkerdData)

	result := parseJSONOutput(t, output)
	assertFieldEquals(t, result, "method", "GET")
	assertFieldEquals(t, result, "url", "http://api-service.test.svc.cluster.local/api/v1/data?id=12345")
	assertFieldEquals(t, result, "timestamp", "2025-09-03T15:30:32.928995068Z")
}

func testHostChangeConversion(t *testing.T) {
	linkerdData := getSampleLinkerdData()[0]
	output := runCLICommand(t, []string{"-host", "localhost:8080"}, linkerdData)

	result := parseJSONOutput(t, output)
	assertFieldEquals(t, result, "url", "http://localhost:8080/api/v1/data?id=12345")
}

func testMultipleLineProcessing(t *testing.T) {
	linkerdLines := getSampleLinkerdData()
	linkerdData := strings.Join(linkerdLines, "\n")
	output := runCLICommand(t, []string{"-host", "staging.test.com:9000"}, linkerdData)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(lines))
	}

	for i, line := range lines {
		result := parseJSONOutput(t, line)
		url := getStringField(t, result, "url", i+1)
		assertContains(t, url, "staging.test.com:9000", i+1)
	}
}

func getSampleLinkerdData() []string {
	return []string{
		`{"client.addr":"192.168.1.100:12345","client.id":"test-service.default.serviceaccount.identity.linkerd.cluster.local","host":"api-service.test.svc.cluster.local","method":"GET","processing_ns":"50000","request_bytes":"0","status":200,"timestamp":"2025-09-03T15:30:32.928995068Z","total_ns":"2500000","trace_id":"abc123","uri":"http://api-service.test.svc.cluster.local/api/v1/data?id=12345","user_agent":"TestClient/1.0","version":"HTTP/2.0"}`,
		`{"client.addr":"192.168.1.101:54321","client.id":"another-service.default.serviceaccount.identity.linkerd.cluster.local","host":"api-service.test.svc.cluster.local","method":"POST","processing_ns":"75000","request_bytes":"256","status":201,"timestamp":"2025-09-03T15:30:33.123456789Z","total_ns":"3000000","trace_id":"def456","uri":"http://api-service.test.svc.cluster.local/api/v1/create","user_agent":"TestClient/2.0","version":"HTTP/2.0"}`,
	}
}

func runCLICommand(t *testing.T, args []string, input string) string {
	cmdArgs := append([]string{"run", "main.go"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = "."
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("CLI command failed: %v\nstderr: %s", err, stderr.String())
	}

	return stdout.String()
}

func parseJSONOutput(t *testing.T, output string) map[string]interface{} {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
	if err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}
	return result
}

func assertFieldEquals(t *testing.T, result map[string]interface{}, field, expected string) {
	if result[field] != expected {
		t.Errorf("expected %s %s, got %v", field, expected, result[field])
	}
}

func getStringField(t *testing.T, result map[string]interface{}, field string, lineNum int) string {
	value, ok := result[field].(string)
	if !ok {
		t.Fatalf("%s is not a string in line %d", field, lineNum)
	}
	return value
}

func assertContains(t *testing.T, actual, expected string, lineNum int) {
	if !strings.Contains(actual, expected) {
		t.Errorf("expected %s in line %d, got %s", expected, lineNum, actual)
	}
}

func TestPipelineIntegration(t *testing.T) {
	linkerdData := getSampleLinkerdData()[0]
	output := runCLICommand(t, []string{"-host", "localhost:8080"}, linkerdData)

	ripleyReq := parseJSONOutput(t, output)
	verifyRequiredRipleyFields(t, ripleyReq)
	assertFieldEquals(t, ripleyReq, "url", "http://localhost:8080/api/v1/data?id=12345")

	t.Logf("Successfully converted Linkerd format to Ripley format:\n%s", output)
}

func verifyRequiredRipleyFields(t *testing.T, ripleyReq map[string]interface{}) {
	requiredFields := []string{"method", "url", "timestamp"}
	for _, field := range requiredFields {
		if _, exists := ripleyReq[field]; !exists {
			t.Errorf("missing required Ripley field: %s", field)
		}
	}
}
