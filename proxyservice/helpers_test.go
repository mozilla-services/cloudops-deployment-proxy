package proxyservice

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
)

type JenkinsJob struct {
	path   string
	params url.Values
}
type FakeJenkins struct {
	Jobs []JenkinsJob
}

func NewFakeJenkins() *FakeJenkins {
	return &FakeJenkins{}
}

func (j *FakeJenkins) TriggerJob(path string, params url.Values) error {
	params.Del("RawJSON")
	j.Jobs = append(j.Jobs, JenkinsJob{path, params})
	return nil
}

func sendRequest(method, url string, body io.Reader, h http.Handler) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	recorder := new(httptest.ResponseRecorder)
	h.ServeHTTP(recorder, req)
	return recorder
}
