package main

import (
	"log"
	"os"

	"github.com/ms11m/smartkubernetes/handler"
)

func main() {
	cfg := handler.Config{
		Port:          getEnv("PORT", "8080"),
		LokiURL:       getEnv("LOKI_URL", "http://loki-stack.monitoring.svc.cluster.local:3100"),
		OllamaURL:     getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "llama3.2:latest"),
		SlackBotToken: os.Getenv("SLACK_BOT_TOKEN"),
		SlackAppToken: os.Getenv("SLACK_APP_TOKEN"),
		SlackChannel:  getEnv("SLACK_CHANNEL", "#neurokube-alerts"),
		KubeConfig:    os.Getenv("KUBECONFIG"),
	}

	srv, err := handler.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[NeuroKube] Brain service starting on :%s", cfg.Port)
	log.Fatal(srv.Start())
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
