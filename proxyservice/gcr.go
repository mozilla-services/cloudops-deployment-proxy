package proxyservice

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
