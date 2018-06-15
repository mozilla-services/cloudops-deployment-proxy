package proxyservice

import (
	"encoding/json"
	"fmt"
	"io"
  "log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type JenkinsCrumbIssuer struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

type Jenkins struct {
	BaseURL string

	User     string
	Password string
}

// NewJenkins returns a new Jenkins instance
func NewJenkins(baseURL, user, password string) *Jenkins {
	return &Jenkins{
		BaseURL:  baseURL,
		User:     user,
		Password: password,
	}
}

// NewRequest builds a authed jenkins request.
// path must be the absolute path starting with "/"
func (j *Jenkins) NewRequest(method, path string, body io.Reader) (*http.Request, error) {
	url := j.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(j.User, j.Password)
	return req, nil
}

func (j *Jenkins) setCSRFToken(req *http.Request) error {
	csrfReq, err := j.NewRequest("GET", "/crumbIssuer/api/json", nil)
	if err != nil {
		return fmt.Errorf("Error building csrf request: %v", err)
	}

	resp, err := http.DefaultClient.Do(csrfReq)
	if err != nil {
		return fmt.Errorf("Error requesting csrf token: %v", err)
	}
	defer resp.Body.Close()

	crumb := new(JenkinsCrumbIssuer)
	if err := json.NewDecoder(resp.Body).Decode(crumb); err != nil {
		return fmt.Errorf("Could not decode err: %v", err)
	}

	req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	return nil
}

// PostForm posts a authed request to jenkins BaseURL + path
func (j *Jenkins) PostForm(path string, data url.Values) (*http.Response, error) {
	req, err := j.NewRequest("POST", path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	if err := j.setCSRFToken(req); err != nil {
		return nil, fmt.Errorf("Could not set CSRF: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return http.DefaultClient.Do(req)
}

// TriggerJob triggers a jenkins job
// jobPath should be the full path to the job e.g., /job/pipelines/job/myjob/
func (j *Jenkins) TriggerJob(jobPath string, params url.Values) error {
	resp, err := j.PostForm(path.Join(jobPath, "buildWithParameters"), params)
	if err != nil {
		return fmt.Errorf("Error posting to jenkins: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("Jenkins returned %d for %s, expected 201", resp.StatusCode, resp.Request.URL)
	}
	return nil
}

// TriggerJenkinsJob triggers a jenkins job
// given docker repo name, docker repo namespace, and image tag
func (j *Jenkins) TriggerJenkinsJob(repoName, namespace, tag, rawJSON string) error {
  log.Printf("Triggering Jenkins Job for: %s %s with tag: %s", repoName, namespace, tag)

	if !regexp.MustCompile(`^[a-zA-Z0-9_\-]{2,255}$`).MatchString(repoName) {
		return fmt.Errorf("Invalid repoName: %s", repoName)
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_\-]{2,255}$`).MatchString(namespace) {
		return fmt.Errorf("Invalid namespace: %s", namespace)
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,100}$`).MatchString(tag) {
		return fmt.Errorf("Invalid tag: %s", tag)
	}

	path := path.Join("/job/dockerhub/job",
		namespace, "job", repoName)
	params := url.Values{}
	params.Set("Tag", tag)
	params.Set("RawJSON", rawJSON)
	return j.TriggerJob(path, params)
}
