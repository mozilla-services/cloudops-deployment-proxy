package proxyservice

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type Jenkins struct {
	BaseURL string

	User     string
	Password string
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

// PostForm posts a authed request to jenkins BaseURL + path
func (j *Jenkins) PostForm(path string, data url.Values) (*http.Response, error) {
	req, err := j.NewRequest("POST", path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
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
		return fmt.Errorf("Jenkins did not returned %d, expected 201", resp.StatusCode)
	}
	return nil
}
