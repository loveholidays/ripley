package converter

import (
	"fmt"
	"net/url"
	"time"

	ripley "github.com/loveholidays/ripley/pkg"
	"github.com/loveholidays/ripley/tools/linkerdxripley/pkg/linkerd"
)

type Converter struct{}

func New() *Converter {
	return &Converter{}
}

func (c *Converter) ConvertToRipley(linkerd linkerd.Request, newHost string, upgradeHTTPS bool) (*ripley.Request, error) {
	timestamp, err := c.parseTimestamp(linkerd.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp %s: %w", linkerd.Timestamp, err)
	}

	targetURL, err := c.buildTargetURL(linkerd.URI, newHost, upgradeHTTPS)
	if err != nil {
		return nil, fmt.Errorf("failed to build target URL: %w", err)
	}

	headers := c.buildHeaders(linkerd)

	ripleyReq := &ripley.Request{
		Method:    linkerd.Method,
		Url:       targetURL,
		Timestamp: timestamp,
		Headers:   headers,
	}

	return ripleyReq, nil
}

func (c *Converter) parseTimestamp(timestampStr string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, timestampStr)
}

func (c *Converter) buildTargetURL(originalURI, newHost string, upgradeHTTPS bool) (string, error) {
	parsedURL, err := url.Parse(originalURI)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI %s: %w", originalURI, err)
	}

	if newHost != "" {
		parsedURL.Host = newHost
	}

	if upgradeHTTPS && parsedURL.Scheme == "http" {
		parsedURL.Scheme = "https"
	}

	return parsedURL.String(), nil
}

func (c *Converter) buildHeaders(linkerd linkerd.Request) map[string]string {
	headers := make(map[string]string)

	if linkerd.UserAgent != "" {
		headers["User-Agent"] = linkerd.UserAgent
	}

	return headers
}
