package proxyservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var fakeJenkins = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/crumbIssuer/api/json" {
		w.Write([]byte(`{"crumb": "crmb", "crumbRequestField": "Jenkins-Crumb"}`))
		return
	}
	w.WriteHeader(201)
}))

var fakeDockerHub = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	if strings.HasSuffix(req.URL.Path, "fail") {
		w.WriteHeader(403)
		return
	}
	w.WriteHeader(200)
}))

func init() {
	DockerhubRegistry = fakeDockerHub.URL
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

type DockerhubFixtureTest struct {
	TestName   string
	StatusCode int
	ModFunc    func(*DockerHubWebhookData) *DockerHubWebhookData
}

func TestDockerHubHandler(t *testing.T) {
	handler := NewDockerHubWebhookHandler(
		NewJenkins(
			fakeJenkins.URL,
			"fakeuser",
			"fakepass",
		),
		"mozilla",
	)

	// Non-POST
	resp := sendRequest("GET", "http://test/dockerhub", nil, handler)
	assert.Equal(t, http.StatusBadRequest, resp.Code)

	// Invalid JSON
	resp = sendRequest("POST", "http://test/dockerhub", strings.NewReader(`{"invalid"`), handler)
	assert.Equal(t, http.StatusInternalServerError, resp.Code)

	// Fixture Tests
	fixtures := []DockerhubFixtureTest{
		// Input Validation
		// Invalid Callback URL
		{
			TestName:   "Faked Callback",
			StatusCode: http.StatusUnauthorized,
			ModFunc: func(data *DockerHubWebhookData) *DockerHubWebhookData {
				data.CallbackURL = fmt.Sprintf("%s/u/maliciousowner/testrepo/hook/2020202020/", fakeDockerHub.URL)
				return data
			},
		},
		// Invalid Namespace
		{
			TestName:   "Invalid Namespace",
			StatusCode: http.StatusUnauthorized,
			ModFunc: func(data *DockerHubWebhookData) *DockerHubWebhookData {
				data.Repository.Namespace = "invalidddd"
				return data
			},
		},
		// Invalid Name
		{
			TestName:   "Invalid Name",
			StatusCode: http.StatusInternalServerError,
			ModFunc: func(data *DockerHubWebhookData) *DockerHubWebhookData {
				data.CallbackURL = fmt.Sprintf("%s/u/mozilla/abkljaiojewiojf[[[[[[{{}}{}{}/hook/2020202020/", fakeDockerHub.URL)
				data.Repository.Name = "abkljaiojewiojf[[[[[[{{}}{}{}"
				return data
			},
		},
		// Invalid Tag
		{
			TestName:   "Invalid Tag",
			StatusCode: http.StatusInternalServerError,
			ModFunc: func(data *DockerHubWebhookData) *DockerHubWebhookData {
				data.CallbackURL = fmt.Sprintf("%s/u/mozilla/testrepo/hook/2020202020/", fakeDockerHub.URL)
				data.PushData.Tag = "v1.1.1[bad]"
				return data
			},
		},

		// Callback Fails
		{
			TestName:   "Failing Callback",
			StatusCode: http.StatusUnauthorized,
			ModFunc: func(data *DockerHubWebhookData) *DockerHubWebhookData {
				data.CallbackURL = fmt.Sprintf("%s/u/mozilla/testrepo/hook/2020202020/fail", fakeDockerHub.URL)
				return data
			},
		},

		// Valid Request
		{
			TestName:   "Valid Request",
			StatusCode: http.StatusOK,
			ModFunc: func(data *DockerHubWebhookData) *DockerHubWebhookData {
				data.CallbackURL = fmt.Sprintf("%s/u/mozilla/testrepo/hook/2020202020/", fakeDockerHub.URL)
				return data
			},
		},
	}
	for _, fixture := range fixtures {
		data := fixture.ModFunc(dockerhubWebhookData())
		dataBytes, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		resp = sendRequest("POST", "http://test/dockerhub", bytes.NewReader(dataBytes), handler)
		assert.Equal(t, fixture.StatusCode, resp.Code, fixture.TestName)
	}
}

func dockerhubWebhookData() *DockerHubWebhookData {
	f, err := os.Open("fixtures/dockerhub_base.json")
	if err != nil {
		panic(err)
	}
	data := new(DockerHubWebhookData)
	err = json.NewDecoder(f).Decode(data)
	if err != nil {
		panic(err)
	}

	return data
}
