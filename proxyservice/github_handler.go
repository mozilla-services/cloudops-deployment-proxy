package proxyservice

import (
	"fmt"
	"log"
	"net"
	"net/http"
)

// GitHubWebhookHandler directs github webhooks to Jenkins
type GitHubWebhookHandler struct {
	Jenkins          *Jenkins
	ValidOrgs        map[string]bool
	SourceRanges     []*net.IPNet
	UseXForwardedFor bool
}

// NewGitHubWebhookHandler creates a new http handler
func NewGitHubWebhookHandler(jenkins *Jenkins, orgs ...string) (*GitHubWebhookHandler, error) {
	validOrgs := make(map[string]bool)
	for _, org := range orgs {
		validOrgs[org] = true
	}
	sourceRanges, err := githubHookSourceRanges()
	if err != nil {
		return nil, fmt.Errorf("githubHookSourceRanges: %s", err)
	}
	return &GitHubWebhookHandler{
		Jenkins:      jenkins,
		SourceRanges: sourceRanges,
		ValidOrgs:    validOrgs,
	}, nil
}

func (d *GitHubWebhookHandler) isValidOrg(org string) bool {
	return d.ValidOrgs[org]
}

func (d *GitHubWebhookHandler) ipFromReq(req *http.Request) string {
	if !d.UseXForwardedFor {
		return req.RemoteAddr
	}
	return req.Header.Get("X-Forwarded-For")
}

func (d *GitHubWebhookHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if reqIP := d.ipFromReq(req); !ipInRanges(reqIP, d.SourceRanges) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		log.Printf("Received POST from unknown IP: %s", reqIP)
		return
	}

	log.Printf("Received github request from: %s", req.RemoteAddr)

	hookData, err := NewGitHubWebhookDataFromRequest(req)
	if err != nil {
		log.Printf("Error parsing request: %v", err)
		http.Error(w, "Internal Service Error", http.StatusInternalServerError)
		return
	}

	if !d.isValidOrg(hookData.payload.Repository.Owner.Name) {
		log.Printf("Invalid Org: %s", hookData.payload.Repository.Owner.Name)
		http.Error(w, "Invalid Org", http.StatusUnauthorized)
		return
	}

	log.Printf("Triggering Jenkins Job for: %s %s with ref: %s",
		hookData.payload.Repository.Owner.Name,
		hookData.payload.Repository.Name,
		hookData.payload.Ref,
	)

	if err := d.Jenkins.TriggerGithubJob(hookData); err != nil {
		log.Printf("Error triggering jenkins: %v", err)
		http.Error(w, "Internal Service Error", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("OK"))
}
