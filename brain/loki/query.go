package loki

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 20 * time.Second

type Client struct {
	BaseURL string
	// HTTP is optional; when nil, a client with defaultHTTPTimeout is used.
	HTTP *http.Client
}

type lokiResponse struct {
	Data struct {
		Result []struct {
			Values [][]string `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func (c *Client) queryRange(query string, lines int) ([]byte, error) {
	params := url.Values{
		"query":     {query},
		"limit":     {fmt.Sprintf("%d", lines)},
		"start":     {fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).UnixNano())},
		"end":       {fmt.Sprintf("%d", time.Now().UnixNano())},
		"direction": {"backward"},
	}
	reqURL := c.BaseURL + "/loki/api/v1/query_range?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("loki query_range: %s — %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func decodeLogLines(body []byte) (string, bool) {
	var result lokiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false
	}
	var sb strings.Builder
	for _, stream := range result.Data.Result {
		for _, entry := range stream.Values {
			if len(entry) >= 2 {
				sb.WriteString(entry[1] + "\n")
			}
		}
	}
	return sb.String(), sb.Len() > 0
}

// FetchLogs pulls recent lines for a pod. Uses exact pod label match first, then pod_name
// (some Promtail / chart versions use pod_name). Alertmanager usually sends the full pod name.
func (c *Client) FetchLogs(namespace, pod string, lines int) (string, error) {
	if strings.TrimSpace(pod) == "" {
		return "(no pod label in alert — skipping log fetch)", nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = "default"
	}

	queries := []string{
		fmt.Sprintf(`{namespace=%q, pod=%q}`, namespace, pod),
		fmt.Sprintf(`{namespace=%q, pod_name=%q}`, namespace, pod),
	}

	for _, query := range queries {
		body, err := c.queryRange(query, lines)
		if err != nil {
			return "", err
		}
		text, ok := decodeLogLines(body)
		if ok {
			return text, nil
		}
	}
	return "(no logs found in last 10 minutes)", nil
}
