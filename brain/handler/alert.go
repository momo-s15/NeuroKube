package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

type AlertmanagerPayload struct {
	Alerts []Alert `json:"alerts"`
}

type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func (s *Server) handleAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload AlertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}

	for _, alert := range payload.Alerts {
		if alert.Status != "firing" {
			continue
		}
		go s.processAlert(alert)
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) processAlert(alert Alert) {
	pod := alert.Labels["pod"]
	namespace := alert.Labels["namespace"]
	alertName := alert.Labels["alertname"]
	log.Printf("[alert] %s fired for %s/%s", alertName, namespace, pod)

	logs, err := s.loki.FetchLogs(namespace, pod, 50)
	if err != nil {
		log.Printf("[loki] error: %v", err)
		logs = "(log fetch failed — proceeding with alert metadata only)"
	}

	diagnosis, err := s.llm.Diagnose(alertName, pod, logs)
	if err != nil {
		log.Printf("[llm] error: %v", err)
		diagnosis = "AI diagnosis unavailable."
	}

	s.slack.SendCrashAlert(namespace, pod, alertName, diagnosis)
}
