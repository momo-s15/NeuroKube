package handler

import (
	"log"
	"net/http"

	"github.com/ms11m/smartkubernetes/k8s"
	"github.com/ms11m/smartkubernetes/llm"
	"github.com/ms11m/smartkubernetes/loki"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	slackgo "github.com/slack-go/slack"
)

type Config struct {
	Port, LokiURL, OllamaURL, OllamaModel      string
	SlackBotToken, SlackAppToken, SlackChannel string
	KubeConfig                                 string
}

type Server struct {
	cfg   Config
	loki  *loki.Client
	llm   *llm.Client
	slack *SlackClient
	k8s   *k8s.Client
}

func NewServer(cfg Config) (*Server, error) {
	s := &Server{cfg: cfg}
	s.loki = &loki.Client{BaseURL: cfg.LokiURL}
	s.llm = llm.NewClient(cfg.OllamaURL, cfg.OllamaModel)

	api := slackgo.New(cfg.SlackBotToken)
	s.slack = NewSlackClient(api, cfg.SlackChannel)

	kc, err := k8s.NewClient(cfg.KubeConfig)
	if err != nil {
		log.Printf("[k8s] client unavailable: %v", err)
	} else {
		s.k8s = kc
	}
	return s, nil
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/alert", s.handleAlert)
	mux.Handle("/metrics", promhttp.Handler())

	if s.cfg.SlackBotToken != "" && s.cfg.SlackAppToken != "" {
		go s.startSocketMode()
	} else {
		log.Printf("[slack] SLACK_BOT_TOKEN / SLACK_APP_TOKEN missing — Socket Mode disabled")
	}

	log.Printf("[NeuroKube] listening on :%s", s.cfg.Port)
	return http.ListenAndServe(":"+s.cfg.Port, mux)
}
