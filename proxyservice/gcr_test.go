package proxyservice

import (
	"bytes"
	"encoding/json"
  "fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type GcrFixtureTest struct {
	TestName   string
	StatusCode int
	ModFunc    func(*pubSubNotification) *pubSubNotification
}

type GcrWebhookDataTest struct {
  TestName string
  InputJSON []byte
  Expected string
  ModFunc func(*GcrWebhookData) interface{}
}

func TestGcrWebhookData(t *testing.T) {
  fixtures := []GcrWebhookDataTest{
    {
      TestName: "Get digest",
      InputJSON: []byte(`{
        "action":"INSERT",
        "digest":"gcr.io/my-project/hello-world@sha256:1d37e48f9ceff6d8030570cd36286a61"
      }`),
      Expected: "sha256:1d37e48f9ceff6d8030570cd36286a61",
      ModFunc: func(d *GcrWebhookData) interface{} {
        return d.getImageTagOrDigest()
      },
    },
    {
      TestName: "Get tag",
      InputJSON: []byte(`{
        "action":"INSERT",
        "tag":"gcr.io/my-project/hello-world:1.1"
      }`),
      Expected: "1.1",
      ModFunc: func(d *GcrWebhookData) interface{} {
        return d.getImageTagOrDigest()
      },
    },
    {
      TestName: "Get repo name",
      InputJSON: []byte(`{
        "action":"INSERT",
        "tag":"gcr.io/my-project/hello-world:1.1"
      }`),
      Expected: "hello-world",
      ModFunc: func(d *GcrWebhookData) interface{} {
        return d.getRepositoryName()
      },
    },
    {
      TestName: "Get domain name",
      InputJSON: []byte(`{
        "action":"INSERT",
        "tag":"gcr.io/my-project/hello-world:1.1"
      }`),
      Expected: "my-project",
      ModFunc: func(d *GcrWebhookData) interface{} {
        return d.getRepositoryDomain()
      },
    },
  }

  for _, fixture := range fixtures {
    var data GcrWebhookData
    err := json.Unmarshal(fixture.InputJSON, &data)
    if err != nil {
      t.Fatal(fmt.Errorf("Error unmarshaling json: %v", err))
    }
    result := fixture.ModFunc(&data)
    assert.Equal(t, fixture.Expected, result, fixture.TestName)
  }
}

func TestGcrHandler(t *testing.T) {
	handler := NewGcrWebhookHandler(
		NewJenkins(
			fakeJenkins.URL,
			"fakeuser",
			"fakepass",
		),
		"mozilla",
	)

	// Invalid JSON
	resp := sendRequest("POST", "http://test/dockerhub", strings.NewReader(`{"invalid"`), handler)
	assert.Equal(t, http.StatusInternalServerError, resp.Code)

	// Fixture Tests
	fixtures := []GcrFixtureTest{
		// Invalid Namespace
		{
			TestName:   "Invalid Action",
			StatusCode: http.StatusInternalServerError,
			ModFunc: func(notification *pubSubNotification) *pubSubNotification {
				notification.Message = pubSubMessage{
					// {"action": "invalidddd"}
					Data: "eyJhY3Rpb24iOiAiaW52YWxpZGRkZCJ9Cg==",
				}
				return notification
			},
		},
	}

	for _, fixture := range fixtures {
		data := fixture.ModFunc(gcrWebhookData())
		dataBytes, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		resp = sendRequest("POST", "http://test/gcr", bytes.NewReader(dataBytes), handler)
		assert.Equal(t, fixture.StatusCode, resp.Code, fixture.TestName)
	}
}

func gcrWebhookData() *pubSubNotification {
	f, err := os.Open("fixtures/gcr_base.json")
	if err != nil {
		panic(err)
	}
	data := new(pubSubNotification)
	err = json.NewDecoder(f).Decode(data)
	if err != nil {
		panic(err)
	}

	return data
}
