package converter

import (
	"fmt"
	"net/url"
	"time"

	"github.com/loveholidays/ripley/tools/linkerdxripley/pkg/linkerd"
	ripley "github.com/loveholidays/ripley/pkg"
)

type Converter struct{}

func New() *Converter {
	return &Converter{}
}

func (c *Converter) ConvertToRipley(linkerd linkerd.Request, newHost string) (*ripley.Request, error) {
	timestamp, err := c.parseTimestamp(linkerd.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp %s: %w", linkerd.Timestamp, err)
	}

	targetURL, err := c.buildTargetURL(linkerd.URI, newHost)
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

func (c *Converter) buildTargetURL(originalURI, newHost string) (string, error) {
	if newHost == "" {
		return originalURI, nil
	}

	parsedURL, err := url.Parse(originalURI)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI %s: %w", originalURI, err)
	}

	parsedURL.Host = newHost
	return parsedURL.String(), nil
}

func (c *Converter) buildHeaders(linkerd linkerd.Request) map[string]string {
	headers := make(map[string]string)

	if linkerd.UserAgent != "" {
		headers["User-Agent"] = linkerd.UserAgent
	}

	if linkerd.Host != "" {
		headers["Host"] = linkerd.Host
	}

	return headers
}