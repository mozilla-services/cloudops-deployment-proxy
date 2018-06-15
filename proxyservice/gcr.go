package proxyservice

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
  "fmt"
	"io/ioutil"
  "net/http"
  "strings"

  "github.com/docker/distribution/reference"
)

type pubSubNotification struct {
	Message      pubSubMessage
	Subscription string
}

type pubSubMessage struct {
	Attributes map[string]string
	Data       string
	MessageId  string
}

type GcrWebhookData struct {
	Action string `json:"action"`
	Digest string `json:"digest,omitempty"`
	Tag    string `json:"tag,omitempty"`
}

func (d *GcrWebhookData) isValid() bool {
	// check for valid actions
	validActions := map[string]bool{"INSERT": true}
	return validActions[d.Action]

  // check for image reference regexp match
  return reference.ReferenceRegexp.MatchString(d.getImageReference())

	return true
}

func (d *GcrWebhookData) getImageReference() string {
  if d.Tag != "" {
    return d.Tag
  } else {
    return d.Digest
  }
}

func (d *GcrWebhookData) getImageTagOrDigest() string {
  if d.Tag != "" {
    return reference.ReferenceRegexp.FindStringSubmatch(d.Tag)[2]
  } else {
    return reference.ReferenceRegexp.FindStringSubmatch(d.Digest)[3]
  }
}

func (d *GcrWebhookData) getRepositoryDomain() string {
  m := reference.ReferenceRegexp.FindStringSubmatch(d.getImageReference())[1]
  return strings.Split(m, "/")[1]
}

func (d *GcrWebhookData) getRepositoryName() string {
  m := reference.ReferenceRegexp.FindStringSubmatch(d.getImageReference())[1]
  return strings.Split(m, "/")[2]
}

func (d *GcrWebhookData) rawJSON() (string, error) {
  rawJSON, err := json.Marshal(d)
  if err != nil {
    return "", fmt.Errorf("Error marshaling data: %v", err)
  }
  return string(rawJSON), nil
}

func newPubSubNotificationFromRequest(req *http.Request) (*pubSubNotification, error) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading request body: %v", err)
	}

	req.Body = ioutil.NopCloser(bytes.NewReader(body))

	var data pubSubNotification
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling json: %v", err)
	}

	return &data, nil
}

func NewGcrWebhookDataFromRequest(req *http.Request) (*GcrWebhookData, error) {
	pubSubNotification, err := newPubSubNotificationFromRequest(req)
	if err != nil {
		return nil, fmt.Errorf("Error parsing PubSub notification: %v", err)
	}

	b64 := pubSubNotification.Message.Data
	dataJson, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("Error base64-decoding PubSub message: %v", err)
	}

	var data GcrWebhookData
	err = json.Unmarshal(dataJson, &data)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling json: %v", err)
	}

	return &data, nil
}
