package proxyservice

import (
	"encoding/json"
	"fmt"
	"io"
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

type JenkinsServer struct {
	BaseURL string

	User     string
	Password string
}

type Jenkins interface {
	TriggerJob(jobPath string, params url.Values) error
}

// NewJenkins returns a new Jenkins instance
func NewJenkins(baseURL, user, password string) Jenkins {
	return &JenkinsServer{
		BaseURL:  baseURL,
		User:     user,
		Password: password,
	}
}

// NewRequest builds a authed jenkins request.
// path must be the absolute path starting with "/"
func (j *JenkinsServer) NewRequest(method, path string, body io.Reader) (*http.Request, error) {
	url := j.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(j.User, j.Password)
	return req, nil
}

func (j *JenkinsServer) setCSRFToken(req *http.Request) error {
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
func (j *JenkinsServer) PostForm(path string, data url.Values) (*http.Response, error) {
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
func (j *JenkinsServer) TriggerJob(jobPath string, params url.Values) error {
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

// TriggerDockerhubJob triggers a jenkins job
// given DockerHubWebhookData
func TriggerDockerhubJob(j Jenkins, data *DockerHubWebhookData) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9_\-]{2,255}$`).MatchString(data.Repository.Name) {
		return fmt.Errorf("Invalid data.Repository.Name: %s", data.Repository.Name)
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_\-]{2,255}$`).MatchString(data.Repository.Namespace) {
		return fmt.Errorf("Invalid data.Repository.Namespace: %s", data.Repository.Namespace)
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,100}$`).MatchString(data.PushData.Tag) {
		return fmt.Errorf("Invalid data.PushData.Tag: %s", data.PushData.Tag)
	}

	rawJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Error marshaling data: %v", err)
	}
	path := path.Join("/job/dockerhub/job",
		data.Repository.Namespace, "job", data.Repository.Name)
	params := url.Values{}
	params.Set("Tag", data.PushData.Tag)
	params.Set("RawJSON", string(rawJSON))
	return j.TriggerJob(path, params)
}

func TriggerHgJob(j Jenkins, repoPath string, repoUrl string, head string, data *HgMessage) error {
	matches := regexp.MustCompile("^([A-Za-z][A-Za-z0-9-]*)/([A-Za-z][A-Za-z0-9-]*)$").FindStringSubmatch(repoPath)
	if len(matches) != 3 {
		return fmt.Errorf("Invalid hg.mozilla.org repository path: %s", repoPath)
	}

	rawJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Error marshaling data: %v", err)
	}
	path := path.Join("/job/hgmo/job", matches[1], "job", matches[2])
	params := url.Values{}
	params.Set("HEAD_REPOSITORY", repoUrl)
	params.Set("HEAD_REV", head)
	params.Set("RawJSON", string(rawJSON))
	return j.TriggerJob(path, params)
}
