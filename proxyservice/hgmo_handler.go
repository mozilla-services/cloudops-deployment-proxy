package proxyservice

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/streadway/amqp"
	"github.com/taskcluster/pulse-go/pulse"
)

// https://mozilla-version-control-tools.readthedocs.io/en/latest/hgmo/notifications.html#pulse-notifications
type HgMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type ChangegroupMessage struct {
	RepoUrl       string   `json:"repo_url"`
	Heads         []string `json:"heads"`
	PushlogPushes []struct {
		PushId          int    `json:"pushid"`
		User            string `json:"user"`
		Time            int    `json:"time"`
		PushJsonUrl     string `json:"push_json_url"`
		PushFullJsonUrl string `json:"push_full_json_url"`
	} `json:"pushlog_pushes"`
	Source string `json:"Source"`
}

func (msg *HgMessage) UnmarshalJSON(b []byte) error {
	// hgmo generates an extra layer of wrapping
	// https://bugzilla.mozilla.org/show_bug.cgi?id=1625386
	var raw struct {
		Payload struct {
			Type string
			Data json.RawMessage
		} `json:"payload"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	msg.Type = raw.Payload.Type
	if msg.Type == "changegroup.1" {
		var data ChangegroupMessage
		if err := json.Unmarshal(raw.Payload.Data, &data); err != nil {
			return err
		}
		msg.Data = data
		return nil
	}
	return fmt.Errorf("Unknown hg message type %s", msg.Type)
}

type PushJson struct {
	Lastpushid int `json:"lastpushid"`
	Pushes     map[int]struct {
		Changesets []string `json:"changesets"`
		Date       int      `json:"date"`
		User       string   `json:"user"`
	} `json:"pushes"`
}

func (msg *ChangegroupMessage) VerifyMessage(repoPath string) error {
	repoUrl := fmt.Sprintf("https://hg.mozilla.org/%s", repoPath)
	if msg.RepoUrl != repoUrl {
		return fmt.Errorf("Message %v has repoUrl %s which doesn't match routing key %s", msg, msg.RepoUrl, repoPath)
	}
	if len(msg.Heads) != 1 {
		return fmt.Errorf("Message %v has %d heads, only 1 supported", msg, len(msg.Heads))
	}
	if len(msg.PushlogPushes) != 1 {
		return fmt.Errorf("Message %v has %d pushlog pushes, only 1 supported", msg, len(msg.PushlogPushes))
	}

	prefix := fmt.Sprintf("%s/json-pushes?version=2&", repoUrl)
	pushJsonUrl := msg.PushlogPushes[0].PushJsonUrl
	if !strings.HasPrefix(pushJsonUrl, prefix) {
		return fmt.Errorf("push_json_url does not start with %s", prefix)
	}

	pushJsonUrl = fmt.Sprintf("%s&tipsonly=1", pushJsonUrl)
	resp, err := http.Get(pushJsonUrl)
	if err != nil {
		return fmt.Errorf("Error calling push_json_url %s: %v", pushJsonUrl, err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("push_json_url %s did not return 200", pushJsonUrl)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading push_json_url response body: %v", err)
	}
	pushJson := PushJson{}
	if err := json.Unmarshal(body, &pushJson); err != nil {
		return fmt.Errorf("Error parsing push_json_url response body: %v %s", err, body)
	}

	msgPush := msg.PushlogPushes[0]
	apiPush, ok := pushJson.Pushes[msgPush.PushId]
	if !ok {
		return fmt.Errorf("Did not find push %d in push_json_url response: %v", msgPush.PushId, pushJson)
	}

	if msgPush.User != apiPush.User || msgPush.Time != apiPush.Date || len(apiPush.Changesets) != 1 || msg.Heads[0] != apiPush.Changesets[0] {
		return fmt.Errorf("push_json_url reponse does not match pulse message: %v %v", msg, pushJson)
	}
	return nil
}

type hgPushBinding struct {
	Repository string
}

func (binding hgPushBinding) RoutingKey() string {
	return binding.Repository
}

func (binding hgPushBinding) ExchangeName() string {
	return "exchange/hgpushes/v2"
}

func (binding hgPushBinding) NewPayloadObject() interface{} {
	return new(HgMessage)
}

type HgmoPulseHandler struct {
	Jenkins      Jenkins
	Pulse        *pulse.Connection
	QueueName    string
	ValidHgRepos map[string]bool
}

func NewHgmoPulseHandler(jenkins Jenkins, pulse *pulse.Connection, queueName string, hgRepos ...string) *HgmoPulseHandler {
	validHgRepos := make(map[string]bool)
	for _, hgRepo := range hgRepos {
		validHgRepos[hgRepo] = true
	}
	log.Print(hgRepos)
	return &HgmoPulseHandler{
		Jenkins:      jenkins,
		Pulse:        pulse,
		QueueName:    queueName,
		ValidHgRepos: validHgRepos,
	}
}

func (handler *HgmoPulseHandler) handleMessage(message interface{}, delivery amqp.Delivery) {
	switch t := message.(type) {
	case *HgMessage:
		switch data := t.Data.(type) {
		case ChangegroupMessage:
			repoPath := delivery.RoutingKey
			if !handler.ValidHgRepos[repoPath] {
				log.Printf("Unwatched repository %s", repoPath)
				break
			}
			if err := data.VerifyMessage(repoPath); err != nil {
				log.Printf("%s", err)
				break
			}
			if err := TriggerHgJob(handler.Jenkins, repoPath, data.RepoUrl, data.Heads[0], t); err != nil {
				log.Printf("Error triggering hg.mozilla.org job: %s", err)
			}
		}
	}
	delivery.Ack(false) // acknowledge message *after* processing
}

func (handler *HgmoPulseHandler) Consume() error {
	bindings := make([]pulse.Binding, 0)
	for validHgRepo := range handler.ValidHgRepos {
		bindings = append(bindings, hgPushBinding{Repository: validHgRepo})
	}
	_, err := handler.Pulse.Consume(
		handler.QueueName,
		handler.handleMessage,
		1,     // prefetch 1 message at a time
		false, // don't autoacknowledge messages
		bindings...,
	)
	return err
}
