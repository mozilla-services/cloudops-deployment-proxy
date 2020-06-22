package pulse

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/pborman/uuid"
	"github.com/streadway/amqp"
)

// Utility method used for checking an error condition, and failing with a given
// error message if the error is not nil. msg should contain a description of
// what activity could not be performed as required.
func (err PulseError) Error() string {
	msg := err.Message
	lle := err.LowerLevelError
	if msg != "" {
		if lle != nil {
			return fmt.Sprintf("%s: %s", msg, lle)
		}
		return fmt.Sprintf("%s", msg)
	}
	if lle != nil {
		return fmt.Sprintf("Pulse library error occurred caused by:\n%s", lle)
	}
	return "Unknown Pulse Library error has occurred! No information available! :D"
}

// Utility function for generating a PulseError
func Error(err error, msg string) PulseError {
	return PulseError{
		Message:         msg,
		LowerLevelError: err,
	}
}

// PulseError is used for describing problems that occurred when interacting
// with Pulse, caused by a lower-level error
type PulseError struct {
	Message         string
	LowerLevelError error
}

// PulseQueue manages an underlying AMQP queue, and provides methods for
// closing, deleting, pausing and resuming queues.
type PulseQueue struct {
}

// Connection manages the underlying AMQP connection, and provides an interface
// for performing further actions, such as creating a queue.
type Connection struct {
	User        string
	Password    string
	URL         string
	AMQPConn    *amqp.Connection
	connected   bool
	closedAlert chan amqp.Error
}

// match applies the regular expression regex to string text, and only replaces
// with $1 if there is a match, otherwise if no match, returns an empty string
func match(regex, text string) string {
	if matched, _ := regexp.MatchString(regex, text); matched {
		return regexp.MustCompile(regex).ReplaceAllString(text, "$1")
	}
	return ""
}

// NewConnection prepares a Connection object with a username, password and an
// AMQP URL, but does not actually make an outbound connection to the service.
// An actual network connection will be made the first time the Consume method
// is called.
//
// The logic for deriving the AMQP url is as follows:
//
// If the provided amqpUrl is a non-empty string, it will be used to set the
// AMQP URL.  Otherwise, production will be used
// ("amqps://pulse.mozilla.org:5671")
//
// The pulse user is determined as follows:
//
// If the provided pulseUser is a non-empty string, it will be used for AMQP
// connection user.  Otherwise, if the amqlUrl contains a user, it will be
// used.  Otherwise, if environment variable PULSE_USERNAME is non empty, it
// will be used.  Otherwise, the value "guest" will be used.
//
// The pulse password is determined as follows:
//
// If the provided pulsePassword is a non-empty string, it will be used for
// AMQP connection password.  Otherwise, if the amqlUrl contains a password, it
// will be used.  Otherwise, if environment variable PULSE_PASSWORD is non
// empty, it will be used.  Otherwise, the value "guest" will be used.
//
// Finally, the AMQP url is adjusted, by stripping out any user/password
// contained inside it, and then embedding the derived username and password
// above.
//
// Typically, a call to this method would look like:
//
//  	conn := pulse.NewConnection("", "", "")
//
// whereby the client program would export PULSE_USERNAME and PULSE_PASSWORD
// environment variables before calling the go program, and the empty url would
// signify that the client should connect to the production instance.
func NewConnection(pulseUser string, pulsePassword string, amqpUrl string) Connection {
	if amqpUrl == "" {
		amqpUrl = "amqps://pulse.mozilla.org:5671"
	}
	if pulseUser == "" {
		// Regular expression to pull out username from amqp url
		pulseUser = match("^.*://([^:@/]*)(:[^@]*@|@).*$", amqpUrl)
	}
	if pulsePassword == "" {
		// Regular expression to pull out password from amqp url
		pulsePassword = match("^.*://[^:@/]*:([^@]*)@.*$", amqpUrl)
	}
	if pulseUser == "" {
		pulseUser = os.Getenv("PULSE_USERNAME")
	}
	if pulsePassword == "" {
		pulsePassword = os.Getenv("PULSE_PASSWORD")
	}
	if pulseUser == "" {
		pulseUser = "guest"
	}
	if pulsePassword == "" {
		pulsePassword = "guest"
	}

	// now substitute in real username and password into url...
	amqpUrl = regexp.MustCompile("^(.*://)([^@/]*@|)([^@]*)(/.*|$)").ReplaceAllString(amqpUrl, "${1}"+pulseUser+":"+pulsePassword+"@${3}${4}")

	return Connection{
		User:     pulseUser,
		Password: pulsePassword,
		URL:      amqpUrl}
}

// connect is called internally, lazily, the first time Consume is called.
// TODO: need to make sure this is properly synchronised.
func (c *Connection) connect() error {
	var err error
	c.AMQPConn, err = amqp.Dial(c.URL)
	if err != nil {
		return Error(err, "Failed to connect to RabbitMQ")
	}
	c.connected = true
	return nil
}

// Binding interface allows you to create custom types to describe exchange /
// routing key combinations. For example Binding types are generated in Task
// Cluster go client to avoid a library user referencing a non existent
// exchange, or an invalid routing key.
type Binding interface {

	// This should return a routing key string for matching pulse messages
	RoutingKey() string

	// This should return the fully qualified name of the pulse exchange to
	// bind messages from
	ExchangeName() string

	// This should return a pointer to a new object for unmarshaling matching
	// pulse messages into
	NewPayloadObject() interface{}
}

// Convenience private (unexported) type for binding a routing key/exchange
// to a queue using plain strings for describing the exchange and routing key
type simpleBinding struct {
	// copy of the static routing key
	rk string
	// copy of the static fully qualified exchange name
	en string
}

// Convenience function for returning a Binding for the given routing key and
// exchange strings, which can be passed to the Consume method of *Connection.
// Typically this is used if you wish to refer to exchanges and routing keys
// with explicit strings, rather than generated types (e.g. Task Cluster go
// client generates custom types to avoid invalid exchange names or invalid
// routing keys).  See the Consume method for more information.
func Bind(routingKey, exchangeName string) Binding {
	return &simpleBinding{rk: routingKey, en: exchangeName}
}

// RoutingKey() blindly returns the routing key the simpleBinding was created
// with in the Bind function above
func (s simpleBinding) RoutingKey() string {
	return s.rk
}

// ExchangeName() blindly returns the exchange name the simpleBinding was
// created with in the Bind function above
func (s simpleBinding) ExchangeName() string {
	return s.en
}

// we unmarshal into an interface{} since we don't know anything about the
// json payload
func (s simpleBinding) NewPayloadObject() interface{} {
	return new(interface{})
}

// Consume is at the heart of the pulse library. After creating a connection
// with pulse.NewConnection(...) above, you can call the Consume method to
// register a queue, set a callback function to be called with each message
// received on the queue and bind the queue to a list of exchange / routing key
// pairs. See the package overview for a walkthrough example. A go routine will
// be spawned to take care of calling the callback function, and a new AMQP
// channel will be created behind-the-scenes to handle the processing.
//
// queueName is the name of the queue to connect to or create; leave empty for
// an anonymous queue that will get auto deleted after disconnecting, or
// provide a name for a long-lived queue.  callback specifies the function to
// be called with each message that arrives.  prefetch specifies how many
// messages should be read from the queue at a time.  autoAck is a bool to
// specify if auto acknowledgements should be sent or not; if not
// auto-acknowledging, remember to ack / nack in your callback method.
// bindings is a variadic input of the exchange names / routing keys that you
// wish pulse to copy to your queue.
func (c *Connection) Consume(
	queueName string,
	callback func(interface{}, amqp.Delivery),
	prefetch int,
	autoAck bool,
	bindings ...Binding,
) (
	PulseQueue,
	error,
) {
	pulseQueue := PulseQueue{}

	// TODO: this needs to be synchronised
	if !c.connected {
		c.connect()
	}

	ch, err := c.AMQPConn.Channel()
	if err != nil {
		return pulseQueue, Error(err, "Failed to open a channel")
	}

	// keep a map from exchange name to exchange object, so later we can
	// unmarshal pulse messages into correct object from the exchange name
	// in the amqp.Delivery object to get back to Binding, and thus to
	// Binding.NewPayloadObject()
	bindingLookup := make(map[string]Binding, len(bindings))

	for i := range bindings {
		err = ch.ExchangeDeclarePassive(
			bindings[i].ExchangeName(), // name
			"topic",                    // type
			false,                      // durable
			false,                      // auto-deleted
			false,                      // internal
			false,                      // no-wait
			nil,                        // arguments
		)
		if err != nil {
			return pulseQueue, Error(err, "Failed to passively declare exchange "+bindings[i].ExchangeName())
		}
		// bookkeeping...
		bindingLookup[bindings[i].ExchangeName()] = bindings[i]
	}

	var q amqp.Queue
	if queueName == "" {
		q, err = ch.QueueDeclare(
			"queue/"+c.User+"/"+uuid.New(), // name
			false, // durable
			// unnamed queues get deleted when disconnected
			true, // delete when usused
			// unnamed queues are exclusive
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
	} else {
		q, err = ch.QueueDeclare(
			"queue/"+c.User+"/"+queueName, // name
			false, // durable
			false, // delete when usused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
	}
	if err != nil {
		return pulseQueue, Error(err, "Failed to declare queue")
	}

	for i := range bindings {
		log.Printf("Binding %s to %s with routing key %s", q.Name, bindings[i].ExchangeName(), bindings[i].RoutingKey())
		err = ch.QueueBind(
			q.Name, // queue name
			bindings[i].RoutingKey(),   // routing key
			bindings[i].ExchangeName(), // exchange
			false,
			nil)
		if err != nil {
			return pulseQueue, Error(err, "Failed to bind a queue")
		}
	}

	eventsChan, err := ch.Consume(
		q.Name,  // queue
		"",      // consumer
		autoAck, // auto ack
		false,   // exclusive
		false,   // no local
		false,   // no wait
		nil,     // args
	)
	if err != nil {
		return pulseQueue, Error(err, "Failed to register a consumer")
	}

	go func() {
		for i := range eventsChan {
			payload := i.Body
			binding, ok := bindingLookup[i.Exchange]
			if !ok {
				panic(errors.New(fmt.Sprintf("ERROR: Message received for an unknown exchange '%v' - not sure how to process", i.Exchange)))
			}
			payloadObject := binding.NewPayloadObject()
			err := json.Unmarshal(payload, payloadObject)
			if err != nil {
				fmt.Printf("Unable to unmarshal json payload into object:\nPayload:\n%v\nObject: %T\n", string(payload), payloadObject)
			}
			callback(payloadObject, i)
		}
		fmt.Println("AMQP channel closed - has the connection dropped?")
	}()
	return pulseQueue, nil
}

// TODO: not yet implemented
func (pq *PulseQueue) Pause() {
}

// TODO: not yet implemented
func (pq *PulseQueue) Delete() {
}

// TODO: not yet implemented
func (pq *PulseQueue) Resume() {
}

// TODO: not yet implemented
func (pq *PulseQueue) Close() {
}
