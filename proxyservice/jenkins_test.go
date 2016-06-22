package proxyservice

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

var fakeJenkinsFailing = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/crumbIssuer/api/json" {
		w.Write([]byte(`{"crumb": "crmb", "crumbRequestField": "Jenkins-Crumb"}`))
		return
	}
	w.WriteHeader(400)
}))

func TestJenkinsTriggerJob(t *testing.T) {
	jenkins := NewJenkins(fakeJenkinsFailing.URL, "fakeuser", "fakepass")
	err := jenkins.TriggerJob("/job/failingjob", url.Values{})
	assert.EqualError(t, err,
		fmt.Sprintf("Jenkins returned 400 for %s/job/failingjob/buildWithParameters, expected 201", fakeJenkinsFailing.URL))
}
