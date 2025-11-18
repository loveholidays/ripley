package converter

import (
	"encoding/json"
	"testing"
	"time"

	ripley "github.com/loveholidays/ripley/pkg"
	"github.com/loveholidays/ripley/tools/linkerdxripley/pkg/linkerd"
)

func TestConverter_ConvertToRipley(t *testing.T) {
	t.Run("successful conversions", func(t *testing.T) {
		testSuccessfulConversions(t)
	})

	t.Run("error cases", func(t *testing.T) {
		testConversionErrors(t)
	})
}

func testSuccessfulConversions(t *testing.T) {
	t.Run("basic conversion tests", func(t *testing.T) {
		testBasicConversions(t)
	})

	t.Run("HTTPS upgrade tests", func(t *testing.T) {
		testHTTPSUpgradeConversions(t)
	})
}

func testBasicConversions(t *testing.T) {
	conv := New()

	tests := []conversionTest{
		{
			name:       "basic conversion without host change",
			linkerdReq: createFullLinkerdRequest(),
			newHost:    "",
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "http://api-service.test.svc.cluster.local/api/v1/data?id=12345",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestClient/1.0",
				},
			},
		},
		{
			name:       "conversion with host change",
			linkerdReq: createSimpleLinkerdRequest("POST", "api.test.com", "http://api.test.com/endpoint", "TestAgent/1.0"),
			newHost:    "localhost:8080",
			expected: &ripley.Request{
				Method:    "POST",
				Url:       "http://localhost:8080/endpoint",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestAgent/1.0",
				},
			},
		},
		{
			name:       "conversion without user agent",
			linkerdReq: createSimpleLinkerdRequest("GET", "api.test.com", "http://api.test.com/endpoint", ""),
			newHost:    "",
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "http://api.test.com/endpoint",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers:   map[string]string{},
			},
		},
		{
			name:       "conversion with complex query parameters",
			linkerdReq: createSimpleLinkerdRequest("GET", "search-api.test.com", "http://search-api.test.com/search?q=test&category=books&page=1&limit=10&sort=price", "TestBrowser/2.0"),
			newHost:    "staging.search-api.test.com:9000",
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "http://staging.search-api.test.com:9000/search?q=test&category=books&page=1&limit=10&sort=price",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestBrowser/2.0",
				},
			},
		},
	}

	runConversionTests(t, conv, tests)
}

func testHTTPSUpgradeConversions(t *testing.T) {
	conv := New()

	tests := []conversionTest{
		{
			name:         "HTTP to HTTPS upgrade without host change",
			linkerdReq:   createSimpleLinkerdRequest("GET", "api.test.com", "http://api.test.com/secure-endpoint", "TestAgent/1.0"),
			newHost:      "",
			upgradeHTTPS: true,
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "https://api.test.com/secure-endpoint",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestAgent/1.0",
				},
			},
		},
		{
			name:         "HTTP to HTTPS upgrade with host change",
			linkerdReq:   createSimpleLinkerdRequest("GET", "api.test.com", "http://api.test.com/secure-endpoint", "TestAgent/1.0"),
			newHost:      "localhost:8443",
			upgradeHTTPS: true,
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "https://localhost:8443/secure-endpoint",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestAgent/1.0",
				},
			},
		},
		{
			name:         "HTTPS request remains HTTPS (no downgrade)",
			linkerdReq:   createSimpleLinkerdRequest("GET", "secure-api.test.com", "https://secure-api.test.com/endpoint", "TestAgent/1.0"),
			newHost:      "localhost:8443",
			upgradeHTTPS: true,
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "https://localhost:8443/endpoint",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestAgent/1.0",
				},
			},
		},
	}

	runConversionTests(t, conv, tests)
}

type conversionTest struct {
	name         string
	linkerdReq   linkerd.Request
	newHost      string
	upgradeHTTPS bool
	expected     *ripley.Request
}

func createFullLinkerdRequest() linkerd.Request {
	return linkerd.Request{
		ClientAddr:   "192.168.1.100:12345",
		ClientID:     "test-service.default.serviceaccount.identity.linkerd.cluster.local",
		Host:         "api-service.test.svc.cluster.local",
		Method:       "GET",
		ProcessingNs: "50000",
		RequestBytes: "0",
		Status:       200,
		Timestamp:    "2025-09-03T15:30:32.928995068Z",
		TotalNs:      "2500000",
		TraceID:      "abc123def456",
		URI:          "http://api-service.test.svc.cluster.local/api/v1/data?id=12345",
		UserAgent:    "TestClient/1.0",
		Version:      "HTTP/2.0",
	}
}

func createSimpleLinkerdRequest(method, host, uri, userAgent string) linkerd.Request {
	return linkerd.Request{
		Method:    method,
		Host:      host,
		Timestamp: "2025-09-03T15:30:32.928995068Z",
		URI:       uri,
		UserAgent: userAgent,
	}
}

func runConversionTests(t *testing.T, conv *Converter, tests []conversionTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.ConvertToRipley(tt.linkerdReq, tt.newHost, tt.upgradeHTTPS)
			assertNoError(t, err)
			assertConversionResult(t, result, tt.expected)
		})
	}
}

func testConversionErrors(t *testing.T) {
	conv := New()

	tests := []struct {
		name       string
		linkerdReq linkerd.Request
		newHost    string
	}{
		{
			name: "conversion with invalid timestamp",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Timestamp: "invalid-timestamp",
				URI:       "http://api.test.com/endpoint",
			},
			newHost: "",
		},
		{
			name: "conversion with invalid URI and new host",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "://invalid-uri",
			},
			newHost: "localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := conv.ConvertToRipley(tt.linkerdReq, tt.newHost, false)
			assertError(t, err)
		})
	}
}

func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	if err == nil {
		t.Errorf("expected error but got none")
	}
}

func assertConversionResult(t *testing.T, result, expected *ripley.Request) {
	if result.Method != expected.Method {
		t.Errorf("Method: got %s, want %s", result.Method, expected.Method)
	}

	if result.Url != expected.Url {
		t.Errorf("URL: got %s, want %s", result.Url, expected.Url)
	}

	if !result.Timestamp.Equal(expected.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", result.Timestamp, expected.Timestamp)
	}

	assertHeadersMatch(t, result.Headers, expected.Headers)
}

func assertHeadersMatch(t *testing.T, result, expected map[string]string) {
	if len(result) != len(expected) {
		t.Errorf("Headers length: got %d, want %d", len(result), len(expected))
		return
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("Header %s: got %s, want %s", k, result[k], v)
		}
	}
}

func TestConverter_parseTimestamp(t *testing.T) {
	conv := New()

	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{"valid RFC3339Nano", "2025-09-03T15:30:32.928995068Z", false},
		{"valid RFC3339", "2025-09-03T15:30:32Z", false},
		{"invalid format", "2025/09/03 15:30:32", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := conv.parseTimestamp(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConverter_buildTargetURL(t *testing.T) {
	conv := New()

	tests := []struct {
		name        string
		originalURI string
		newHost     string
		expected    string
		expectError bool
	}{
		{
			name:        "no host change",
			originalURI: "http://test.com/path",
			newHost:     "",
			expected:    "http://test.com/path",
			expectError: false,
		},
		{
			name:        "host change with port",
			originalURI: "http://test.com/path?query=value",
			newHost:     "localhost:8080",
			expected:    "http://localhost:8080/path?query=value",
			expectError: false,
		},
		{
			name:        "invalid URI",
			originalURI: "://invalid",
			newHost:     "localhost",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.buildTargetURL(tt.originalURI, tt.newHost, false)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestConverter_buildHeaders(t *testing.T) {
	conv := New()

	tests := []struct {
		name     string
		linkerd  linkerd.Request
		expected map[string]string
	}{
		{
			name: "user agent present",
			linkerd: linkerd.Request{
				UserAgent: "TestAgent/1.0",
				Host:      "test.com",
			},
			expected: map[string]string{
				"User-Agent": "TestAgent/1.0",
			},
		},
		{
			name: "only user agent",
			linkerd: linkerd.Request{
				UserAgent: "TestAgent/1.0",
				Host:      "",
			},
			expected: map[string]string{
				"User-Agent": "TestAgent/1.0",
			},
		},
		{
			name: "no user agent with host",
			linkerd: linkerd.Request{
				UserAgent: "",
				Host:      "test.com",
			},
			expected: map[string]string{},
		},
		{
			name: "no headers",
			linkerd: linkerd.Request{
				UserAgent: "",
				Host:      "",
			},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.buildHeaders(tt.linkerd)
			assertHeadersMatch(t, result, tt.expected)
		})
	}
}

func TestJSONSerialization(t *testing.T) {
	conv := New()

	linkerdReq := linkerd.Request{
		Method:    "GET",
		Host:      "api.test.com",
		Timestamp: "2025-09-03T15:30:32.928995068Z",
		URI:       "http://api.test.com/endpoint",
		UserAgent: "TestAgent/1.0",
	}

	ripleyReq, err := conv.ConvertToRipley(linkerdReq, "", false)
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	jsonData, err := json.Marshal(ripleyReq)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var unmarshaled ripley.Request
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if unmarshaled.Method != ripleyReq.Method {
		t.Errorf("Method after JSON round-trip: got %s, want %s", unmarshaled.Method, ripleyReq.Method)
	}

	if unmarshaled.Url != ripleyReq.Url {
		t.Errorf("URL after JSON round-trip: got %s, want %s", unmarshaled.Url, ripleyReq.Url)
	}
}

func mustParseTime(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		panic(err)
	}
	return t
}
