package handler

import (
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

func (s *Server) startSocketMode() {
	api := slack.New(s.cfg.SlackBotToken, slack.OptionAppLevelToken(s.cfg.SlackAppToken))
	client := socketmode.New(api)

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeInteractive:
				if evt.Request != nil {
					client.Ack(*evt.Request)
				}
				ev := evt
				go s.handleInteraction(ev)
			}
		}
	}()

	client.Run()
}

func (s *Server) handleInteraction(evt socketmode.Event) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		log.Printf("[slack] unexpected interactive payload type %T", evt.Data)
		return
	}
	actions := callback.ActionCallback.BlockActions
	if len(actions) == 0 {
		return
	}
	action := actions[0]
	s.handleButtonAction(action.ActionID, action.Value, callback.ResponseURL)
}

func (s *Server) handleButtonAction(actionID, value, responseURL string) {
	parts := strings.Split(value, "|")
	if len(parts) < 3 {
		return
	}
	namespace, pod, newLimit := parts[0], parts[1], parts[2]

	switch actionID {
	case "apply_patch":
		if s.k8s == nil {
			_ = s.slack.PostResponseURL(responseURL, "❌ Kubernetes client not configured.")
			return
		}
		if newLimit == "" {
			newLimit = "512Mi"
		}
		log.Printf("[action] patching %s/%s → memory limit %s", namespace, pod, newLimit)
		err := s.k8s.PatchMemoryLimit(namespace, pod, newLimit)
		if err != nil {
			_ = s.slack.PostResponseURL(responseURL, "❌ Patch failed: "+err.Error())
			return
		}
		_ = s.slack.PostResponseURL(responseURL,
			"✅ Patch applied. Pod `"+pod+"` rollout should pick up new nginx memory limit: "+newLimit)

	case "dismiss":
		_ = s.slack.PostResponseURL(responseURL, "Alert dismissed. No changes made.")
	}
}
