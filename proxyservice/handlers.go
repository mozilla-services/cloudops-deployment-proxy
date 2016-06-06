package proxyservice

import (
	"log"
	"net/http"
)

type DockerHubWebhookHandler struct {
}

func NewDockerHubWebhookHandler() *DockerHubWebhookHandler {
	return &DockerHubWebhookHandler{}
}

func (d *DockerHubWebhookHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	hookData, err := NewDockerHubWebhookDataFromRequest(req)
	if err != nil {
		log.Printf("Error parsing request: %v", err)
		http.Error(w, "Internal Service Error", http.StatusInternalServerError)
		return
	}

	if err := hookData.Callback(NewSuccessCallbackData()); err != nil {
		log.Printf("Callback error: %v", err)
		http.Error(w, "Request could not be validated", http.StatusUnauthorized)
		return
	}

	if err := TriggerPipeline(hookData); err != nil {
		log.Printf("Error triggering pipeline: %v", err)
	}
}
