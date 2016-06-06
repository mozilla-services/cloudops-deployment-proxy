package proxyservice

import (
	"log"
	"net/http"
)

type DockerHubWebhookHandler struct {
	Jenkins *Jenkins
}

func NewDockerHubWebhookHandler(jenkins *Jenkins) *DockerHubWebhookHandler {
	return &DockerHubWebhookHandler{
		Jenkins: jenkins,
	}
}

func (d *DockerHubWebhookHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	log.Printf("Received dockerhub request from: %s", req.RemoteAddr)

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

	log.Printf("Triggering Jenkins Job for: %s %s with tag: %s",
		hookData.Repository.Namespace,
		hookData.Repository.Name,
		hookData.PushData.Tag,
	)

	if err := d.Jenkins.TriggerDockerhubJob(hookData); err != nil {
		log.Printf("Error triggering jenkins: %v", err)
	}
}
