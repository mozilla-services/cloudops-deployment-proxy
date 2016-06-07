package proxyservice

import (
	"log"
	"net/http"
)

type DockerHubWebhookHandler struct {
	Jenkins         *Jenkins
	ValidNameSpaces map[string]bool
}

func NewDockerHubWebhookHandler(jenkins *Jenkins, nameSpaces ...string) *DockerHubWebhookHandler {
	validNameSpaces := make(map[string]bool)
	for _, nameSpace := range nameSpaces {
		validNameSpaces[nameSpace] = true
	}
	return &DockerHubWebhookHandler{
		Jenkins:         jenkins,
		ValidNameSpaces: validNameSpaces,
	}
}

func (d *DockerHubWebhookHandler) isValidNamespace(nameSpace string) bool {
	return d.ValidNameSpaces[nameSpace]
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

	if !d.isValidNamespace(hookData.Repository.Namespace) {
		log.Printf("Invalid Namespace: %s", hookData.Repository.Namespace)
		http.Error(w, "Invalid Namespace", http.StatusUnauthorized)
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
