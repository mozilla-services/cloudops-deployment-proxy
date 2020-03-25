package proxyservice

import (
	"github.com/streadway/amqp"

	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

type HgmoFixture struct {
	TestName   string
	RoutingKey string
	ModFunc    func(*HgMessage)
	Message    []byte
	// Results
	Jobs   []JenkinsJob
	Errors []string
}

func TestHgmoHandler(t *testing.T) {
	var logs LogCapture
	defer logs.Reset()
	logs.Start()

	jenkins := NewFakeJenkins()
	handler := NewHgmoPulseHandler(
		jenkins,
		nil,
		"proxy-queue",
		// HG Repos
		"ci/ci-admin",
		"mozilla-central",
		"users/mozilla_hocat.ca/hg-extra",
	)

	fixtures := []HgmoFixture{
		{
			TestName:   "Valid Messsage",
			RoutingKey: "ci/ci-admin",
			Jobs: []JenkinsJob{{
				"/job/hgmo/job/ci/job/ci-admin", url.Values{
					"HEAD_REPOSITORY": {"https://hg.mozilla.org/ci/ci-admin"},
					"HEAD_REV":        {"9c9a898b351909b2e0fe8420ac9d649ded523af3"},
				}}},
			Message: loadFixture("fixtures/hgmo_changegroup.json"),
		},
		{
			TestName:   "Nested Repository",
			RoutingKey: "users/mozilla_hocat.ca/hg-extra",
			Message:    loadFixture("fixtures/hgmo_changegroup.json"),
			Jobs:       nil,
			Errors:     []string{"Invalid hg.mozilla.org repository path"},
		},
		{
			TestName:   "Top-level Repository",
			RoutingKey: "mozilla-central",
			Message:    loadFixture("fixtures/hgmo_changegroup.json"),
			Jobs:       nil,
			Errors:     []string{"Invalid hg.mozilla.org repository path"},
		},
		{
			TestName:   "Multiple Heads",
			RoutingKey: "ci/ci-admin",
			Message:    loadFixture("fixtures/hgmo_changegroup.json"),
			ModFunc: func(message *HgMessage) {
				data := message.Data.(ChangegroupMessage)
				data.Heads = append(data.Heads, "ce7ae9a988cd41bf9ab5852ba8923cc43e62ea34")
				message.Data = data
			},
			Jobs:   nil,
			Errors: []string{"has 2 heads, only 1 supported"},
		},
		{
			TestName:   "Multiple Pushes",
			RoutingKey: "ci/ci-admin",
			Message:    loadFixture("fixtures/hgmo_changegroup.json"),
			ModFunc: func(message *HgMessage) {
				data := message.Data.(ChangegroupMessage)
				data.PushlogPushes = append(data.PushlogPushes, data.PushlogPushes[0])
				message.Data = data
			},
			Jobs:   nil,
			Errors: []string{"has 2 pushlog pushes, only 1 supported"},
		},
		{
			TestName:   "Unknown repo",
			RoutingKey: "ci/taskgraph",
			Message:    loadFixture("fixtures/hgmo_changegroup.json"),
			Jobs:       nil,
			Errors:     []string{"Unwatched repository"},
		},
	}

	for _, fixture := range fixtures {
		jenkins.Jobs = nil
		logs.Messages.Truncate(0)
		t.Run(fixture.TestName, func(t *testing.T) {
			var m HgMessage
			err := json.Unmarshal(fixture.Message, &m)
			if err != nil {
				t.Error(err)
			}
			if fixture.ModFunc != nil {
				fixture.ModFunc(&m)
			}
			handler.handleMessage(&m, amqp.Delivery{
				RoutingKey: fixture.RoutingKey,
			})
			assert.Equal(t, fixture.Jobs, jenkins.Jobs, fixture.TestName)
			for _, error := range fixture.Errors {
				assert.Regexp(t, error, logs.Messages.String(), fixture.TestName)
			}
		})
	}
}
