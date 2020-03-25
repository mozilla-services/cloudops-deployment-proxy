package proxyservice

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/streadway/amqp"
	"github.com/taskcluster/pulse-go/pulse"
	"github.com/taskcluster/taskcluster/clients/client-go/v23/tcqueue"
	"github.com/taskcluster/taskcluster/clients/client-go/v23/tcqueueevents"
)

type TaskCompletedMessage = tcqueueevents.TaskCompletedMessage

type routeTaskCompleted struct {
	Route string
}

func (binding routeTaskCompleted) RoutingKey() string {
	return fmt.Sprintf("route.%s", binding.Route)
}

func (binding routeTaskCompleted) ExchangeName() string {
	return "exchange/taskcluster-queue/v1/task-completed"
}

func (binding routeTaskCompleted) NewPayloadObject() interface{} {
	return new(tcqueueevents.TaskCompletedMessage)
}

type DeployExtra struct {
	ImageTaskId string `json:"image-task-id"`
	Variant     string `json:"variant"`
}

func ParseExtra(extra *json.RawMessage) (DeployExtra, error) {
	var raw struct {
		Deploy DeployExtra `json:"cloudops-deploy"`
	}
	if err := json.Unmarshal(*extra, &raw); err != nil {
		return DeployExtra{}, err
	}
	return raw.Deploy, nil
}

type TaskclusterPulseHandler struct {
	Jenkins          Jenkins
	Pulse            *pulse.Connection
	QueueName        string
	PulseRoutePrefix string
	TcQueue          *tcqueue.Queue
}

func NewTaskclusterPulseHandler(jenkins Jenkins, pulse *pulse.Connection, queueName string, routePrefix string, taskclusterRootUrl string) *TaskclusterPulseHandler {
	return &TaskclusterPulseHandler{
		Jenkins:          jenkins,
		Pulse:            pulse,
		QueueName:        queueName,
		PulseRoutePrefix: routePrefix,
		TcQueue:          tcqueue.New(nil, taskclusterRootUrl),
	}
}

func (handler *TaskclusterPulseHandler) handleMessage(message interface{}, delivery amqp.Delivery) {
	routingKeyPrefix := "route." + handler.PulseRoutePrefix
	switch t := message.(type) {
	case *tcqueueevents.TaskCompletedMessage:
		if strings.HasPrefix(delivery.RoutingKey, routingKeyPrefix) {
			route := strings.TrimPrefix(delivery.RoutingKey, routingKeyPrefix)
			task, err := handler.TcQueue.Task(t.Status.TaskID)
			if err == nil {
				log.Printf("Error triggering taskcluster job: %s", err)
			}
			deploy, err := ParseExtra(&task.Extra)
			if err == nil {
				log.Printf("Error triggering taskcluster job: %s", err)
			}
			if err := TriggerTaskclusterJob(handler.Jenkins, route, deploy, t); err != nil {
				log.Printf("Error triggering taskcluster job: %s", err)
			}
		}
	}
	delivery.Ack(false) // acknowledge message *after* processing

}

func (handler *TaskclusterPulseHandler) Consume() error {
	_, err := handler.Pulse.Consume(
		handler.QueueName,
		handler.handleMessage,
		1,     // prefetch 1 message at a time
		false, // don't autoacknowledge messages
		routeTaskCompleted{Route: handler.PulseRoutePrefix + ".#"},
	)
	return err
}
