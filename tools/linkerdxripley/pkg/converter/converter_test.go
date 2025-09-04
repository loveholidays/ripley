package converter

import (
	"encoding/json"
	"testing"
	"time"

	ripley "github.com/loveholidays/ripley/pkg"
	"github.com/loveholidays/ripley/tools/linkerdxripley/pkg/linkerd"
)

func TestConverter_ConvertToRipley(t *testing.T) {
	conv := New()

	tests := []struct {
		name         string
		linkerdReq   linkerd.Request
		newHost      string
		upgradeHTTPS bool
		expectError  bool
		expected     *ripley.Request
	}{
		{
			name: "basic conversion without host change",
			linkerdReq: linkerd.Request{
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
			},
			newHost:      "",
			upgradeHTTPS: false,
			expectError:  false,
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
			name: "conversion with host change",
			linkerdReq: linkerd.Request{
				Method:    "POST",
				Host:      "api.test.com",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "http://api.test.com/endpoint",
				UserAgent: "TestAgent/1.0",
			},
			newHost:     "localhost:8080",
			expectError: false,
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
			name: "conversion without user agent",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Host:      "api.test.com",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "http://api.test.com/endpoint",
				UserAgent: "",
			},
			newHost:     "",
			expectError: false,
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "http://api.test.com/endpoint",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers:   map[string]string{},
			},
		},
		{
			name: "conversion without host",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Host:      "",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "http://api.test.com/endpoint",
				UserAgent: "TestAgent/1.0",
			},
			newHost:     "",
			expectError: false,
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "http://api.test.com/endpoint",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestAgent/1.0",
				},
			},
		},
		{
			name: "conversion with invalid timestamp",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Timestamp: "invalid-timestamp",
				URI:       "http://api.test.com/endpoint",
			},
			newHost:     "",
			expectError: true,
			expected:    nil,
		},
		{
			name: "conversion with invalid URI and new host",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "://invalid-uri",
			},
			newHost:     "localhost:8080",
			expectError: true,
			expected:    nil,
		},
		{
			name: "conversion with complex query parameters",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Host:      "search-api.test.com",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "http://search-api.test.com/search?q=test&category=books&page=1&limit=10&sort=price",
				UserAgent: "TestBrowser/2.0",
			},
			newHost:      "staging.search-api.test.com:9000",
			upgradeHTTPS: false,
			expectError:  false,
			expected: &ripley.Request{
				Method:    "GET",
				Url:       "http://staging.search-api.test.com:9000/search?q=test&category=books&page=1&limit=10&sort=price",
				Timestamp: mustParseTime("2025-09-03T15:30:32.928995068Z"),
				Headers: map[string]string{
					"User-Agent": "TestBrowser/2.0",
				},
			},
		},
		{
			name: "HTTP to HTTPS upgrade without host change",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Host:      "api.test.com",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "http://api.test.com/secure-endpoint",
				UserAgent: "TestAgent/1.0",
			},
			newHost:      "",
			upgradeHTTPS: true,
			expectError:  false,
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
			name: "HTTP to HTTPS upgrade with host change",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Host:      "api.test.com",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "http://api.test.com/secure-endpoint",
				UserAgent: "TestAgent/1.0",
			},
			newHost:      "localhost:8443",
			upgradeHTTPS: true,
			expectError:  false,
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
			name: "HTTPS request remains HTTPS (no downgrade)",
			linkerdReq: linkerd.Request{
				Method:    "GET",
				Host:      "secure-api.test.com",
				Timestamp: "2025-09-03T15:30:32.928995068Z",
				URI:       "https://secure-api.test.com/endpoint",
				UserAgent: "TestAgent/1.0",
			},
			newHost:      "localhost:8443",
			upgradeHTTPS: true,
			expectError:  false,
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.ConvertToRipley(tt.linkerdReq, tt.newHost, tt.upgradeHTTPS)

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

			if result.Method != tt.expected.Method {
				t.Errorf("Method: got %s, want %s", result.Method, tt.expected.Method)
			}

			if result.Url != tt.expected.Url {
				t.Errorf("URL: got %s, want %s", result.Url, tt.expected.Url)
			}

			if !result.Timestamp.Equal(tt.expected.Timestamp) {
				t.Errorf("Timestamp: got %v, want %v", result.Timestamp, tt.expected.Timestamp)
			}

			if len(result.Headers) != len(tt.expected.Headers) {
				t.Errorf("Headers length: got %d, want %d", len(result.Headers), len(tt.expected.Headers))
			}

			for k, v := range tt.expected.Headers {
				if result.Headers[k] != v {
					t.Errorf("Header %s: got %s, want %s", k, result.Headers[k], v)
				}
			}
		})
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
			name: "both headers present",
			linkerd: linkerd.Request{
				UserAgent: "TestAgent/1.0",
				Host:      "test.com",
			},
			expected: map[string]string{
				"User-Agent": "TestAgent/1.0",
				"Host":       "test.com",
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
			name: "only host",
			linkerd: linkerd.Request{
				UserAgent: "",
				Host:      "test.com",
			},
			expected: map[string]string{
				"Host": "test.com",
			},
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

			if len(result) != len(tt.expected) {
				t.Errorf("Headers length: got %d, want %d", len(result), len(tt.expected))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Header %s: got %s, want %s", k, result[k], v)
				}
			}
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
