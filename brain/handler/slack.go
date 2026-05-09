package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	slackgo "github.com/slack-go/slack"
)

type Diagnosis struct {
	RootCause string `json:"root_cause"`
	Evidence  string `json:"evidence"`
	Fix       string `json:"fix"`
	NewLimit  string `json:"new_limit"`
}

type SlackClient struct {
	api     *slackgo.Client
	channel string
}

func NewSlackClient(api *slackgo.Client, channel string) *SlackClient {
	return &SlackClient{api: api, channel: channel}
}

func extractDiagnosisJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "json")
		s = strings.TrimSpace(s)
		if i := strings.Index(s, "```"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return s
	}
	return s[i : j+1]
}

func (s *SlackClient) SendCrashAlert(ns, pod, alertName, rawDiagnosis string) {
	trimmed := strings.TrimSpace(rawDiagnosis)
	var d Diagnosis
	if err := json.Unmarshal([]byte(extractDiagnosisJSON(trimmed)), &d); err != nil {
		d = Diagnosis{RootCause: trimmed, Fix: "Manual investigation required."}
	}

	evidence := d.Evidence
	if strings.TrimSpace(evidence) == "" {
		evidence = "_(no log line cited)_"
	}

	actionValue := fmt.Sprintf("%s|%s|%s", ns, pod, d.NewLimit)

	blocks := []slackgo.Block{
		slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
			"plain_text", "⚠️ KUBERNETES CRASH DETECTED", false, false)),

		slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
			fmt.Sprintf("*Pod:* `%s/%s`\n*Alert:* `%s`", ns, pod, alertName),
			false, false), nil, nil),

		slackgo.NewDividerBlock(),

		slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
			"*Root Cause:*\n"+d.RootCause, false, false), nil, nil),

		slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
			"*Evidence:*\n"+evidence, false, false), nil, nil),

		slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
			"*Recommended Fix:*\n"+d.Fix, false, false), nil, nil),

		slackgo.NewActionBlock("",
			slackgo.NewButtonBlockElement(
				"apply_patch",
				actionValue,
				slackgo.NewTextBlockObject("plain_text", "✅ Apply Patch", false, false),
			).WithStyle(slackgo.StylePrimary),
			slackgo.NewButtonBlockElement(
				"dismiss",
				actionValue,
				slackgo.NewTextBlockObject("plain_text", "✖ Dismiss", false, false),
			).WithStyle(slackgo.StyleDanger),
		),
	}

	_, _, err := s.api.PostMessage(s.channel, slackgo.MsgOptionBlocks(blocks...))
	if err != nil {
		log.Printf("[slack] post error: %v", err)
	}
}

// PostResponseURL sends an ephemeral confirmation using the interaction response_url.
func (s *SlackClient) PostResponseURL(responseURL, text string) error {
	body, err := json.Marshal(map[string]any{
		"text":             text,
		"response_type":    "ephemeral",
		"replace_original": false,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("response_url POST: %s", resp.Status)
	}
	return nil
}
