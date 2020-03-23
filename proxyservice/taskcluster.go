package proxyservice

import (
	"fmt"
	"log"
	"strings"

	"github.com/streadway/amqp"
	"github.com/taskcluster/pulse-go/pulse"
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

type TaskclusterPulseHandler struct {
	Jenkins          *Jenkins
	Pulse            *pulse.Connection
	PulseRoutePrefix string
}

func NewTaskclusterPulseHandler(jenkins *Jenkins, pulse *pulse.Connection, routePrefix string) *TaskclusterPulseHandler {
	return &TaskclusterPulseHandler{
		Jenkins:          jenkins,
		Pulse:            pulse,
		PulseRoutePrefix: routePrefix,
	}
}

func (handler *TaskclusterPulseHandler) handleMessage(message interface{}, delivery amqp.Delivery) {
	routingKeyPrefix := "route." + handler.PulseRoutePrefix
	switch t := message.(type) {
	case *tcqueueevents.TaskCompletedMessage:
		if strings.HasPrefix(delivery.RoutingKey, routingKeyPrefix) {
			route := strings.TrimPrefix(delivery.RoutingKey, routingKeyPrefix)
			if err := handler.Jenkins.TriggerTaskclusterJob(t.Status.TaskID, route, t); err != nil {
				log.Printf("Error triggering taskcluster job: %s", err)
			}
		}
	}
	delivery.Ack(false) // acknowledge message *after* processing

}

func (handler *TaskclusterPulseHandler) Consume() error {
	routingKeyPrefix := "route." + handler.PulseRoutePrefix
	_, err := handler.Pulse.Consume(
		"", // queue name
		handler.handleMessage,
		1,     // prefetch 1 message at a time
		false, // don't autoacknowledge messages
		routeTaskCompleted{Route: routingKeyPrefix + ".#"},
	)
	return err
}
