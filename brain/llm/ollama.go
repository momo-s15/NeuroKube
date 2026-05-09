package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	Model   string
	http    *http.Client
}

func NewClient(baseURL, model string) *Client {
	return &Client{
		BaseURL: baseURL,
		Model:   model,
		http:    &http.Client{Timeout: 3 * time.Minute},
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

const systemPrompt = `You are an expert Kubernetes SRE.
You receive a Kubernetes pod crash alert and its recent logs.
Respond in exactly this JSON format, nothing else:
{
  "root_cause": "one sentence — the exact technical reason for the crash",
  "evidence":   "the specific log line or metric that confirms it",
  "fix":        "one sentence — the exact remediation command or config change",
  "new_limit":  "if OOMKilled: the recommended new memory limit (e.g. 512Mi), else null"
}`

func (c *Client) Diagnose(alertName, pod, logs string) (string, error) {
	userPrompt := fmt.Sprintf(
		"Alert: %s\nPod: %s\nLogs:\n%s",
		alertName, pod, logs)

	body, err := json.Marshal(ollamaRequest{
		Model:  c.Model,
		Prompt: systemPrompt + "\n\nUser:\n" + userPrompt,
		Stream: false,
	})
	if err != nil {
		return "", err
	}

	resp, err := c.http.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Response, nil
}
