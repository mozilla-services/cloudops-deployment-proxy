// Package pulse provides operations for consuming mozilla pulse messages (see
// https://pulse.mozilla.org/).
//
// For users that are interested in publishing messages, or having lower level
// control of the amqp interactions with pulse, take a look at
// http://godoc.org/github.com/streadway/amqp.  This library is built on top of
// the amqp package.
//
// For a user that is simply interesting in consuming pulse messages without
// wishing to acquire a detailed understanding of how pulse.mozilla.org has
// been designed, or how AMQP 0.9.1 works, this client provides basic utility
// methods to get you started off quickly.
//
// Please note that parent package "github.com/taskcluster/pulse-go" provides a
// very simple command line interface into this library too, which can be
// called directly from a shell, for example, so that the user requires no go
// programming expertise, and can directly write e.g. shell scripts that
// process pulse messages.
//
// To get started, we have created an example program which uses this library.
// The source code for this example is available at
// https://github.com/taskcluster/pulse-go/blob/master/pulsesniffer/pulsesniffer.go.
// Afterwards, we will describe how it works. Do not worry if none of it makes
// sense now.  By the end of this overview it will all be explained.
//
//  // Package pulsesniffer provides a simple example program that listens to some
//  // real world pulse messages.
//  package main
//
//  import (
//  	"fmt"
//  	"github.com/taskcluster/pulse-go/pulse"
//  	"github.com/streadway/amqp"
//  )
//
//  func main() {
//  	// Passing all empty strings:
//  	// empty user => use PULSE_USERNAME env var
//  	// empty password => use PULSE_PASSWORD env var
//  	// empty url => connect to production
//  	conn := pulse.NewConnection("", "", "")
//  	conn.Consume(
//  		"taskprocessing", // queue name
//  		func(message interface{}, delivery amqp.Delivery) { // callback function to pass messages to
//  			fmt.Println("Received from exchange " + delivery.Exchange + ":")
//  			fmt.Println(string(delivery.Body))
//  			fmt.Println("")
//  			delivery.Ack(false) // acknowledge message *after* processing
//  		},
//  		1,     // prefetch 1 message at a time
//  		false, // don't auto-acknowledge messages
//  		pulse.Bind( // routing key and exchange to get messages from
//  			"*.*.*.*.*.*.gaia.#",
//  			"exchange/taskcluster-queue/v1/task-defined"),
//  		pulse.Bind( // another routing key and exchange to get messages from
//  			"*.*.*.*.*.aws-provisioner.#",
//  			"exchange/taskcluster-queue/v1/task-running"))
//  	conn.Consume( // a second workflow to manage concurrently
//  		"", // empty name implies anonymous queue
//  		func(message interface{}, delivery amqp.Delivery) { // simpler callback than before
//  			fmt.Println("Buildbot message received")
//  			fmt.Println("")
//  		},
//  		1,    // prefetch
//  		true, // auto acknowledge, so no need to call delivery.Ack
//  		pulse.Bind( // routing key and exchange to get messages from
//  			"#", // get *all* normalized buildbot messages
//  			"exchange/build/normalized"))
//  	// wait forever
//  	forever := make(chan bool)
//  	<-forever
//  }
// The first thing we need to do is provide connection details for connecting
// to the pulse server, which we do like this:
//
//  conn := pulse.NewConnection("", "", "")
//
// In this example, the provided strings (username, password, url) have all
// been left empty. This is because by default, if you provide no username or
// password, the NewConnection function will inspect environment variables
// PULSE_USERNAME and PULSE_PASSWORD, and an empty url will trigger the library
// to use the current production url. Another example call could be:
//
//  conn := pulse.NewConnection("guest", "guest", "amqp://localhost:5672/")
//
// Typically we would set the username and password credentials via environment
// variables to avoid hardcoding them in the go code.  For more details about
// managing the username, password and amqp url, see the documentation for the
// NewConnection function.
//
// A call to NewConnection does not actually create a connection to the pulse
// server, it simply prepares the data that will be needed when we finally make
// the connection.  Users and passwords can be created by going to the Pulse
// Guardian (https://pulse.mozilla.org) and registering an account.
//
// You will see in the code above, that after creating a connection, there is
// only one more method we call - Consume - which we use for processing
// messages. This is the heart of the pulse library, and where all of the
// action happens.
//
// In pulse, all messages are delivered to "topic exchanges" and the way to
// receive these messages is to request the ones you are interested in are
// copied onto a queue you can read from, and then to read them from the queue.
// This is called binding. To bind messages from an exchange to a queue, you
// specify the name of the exchange you want to receive messages from, and a
// matching criteria to define the ones you want. The matching process is
// handled by routing keys, which will now be explained.
//
// Each message that arrives on an exchange has a "routing key" signature. The
// routing key comprises of several fields. For an example, see:
// https://docs.taskcluster.net/reference/platform/queue/exchanges#taskDefined.
// The fields are delimited by dots, and therefore the routing key of a message
// is represented as a '.' delimited string. In order to select the messages
// on an exchange that you wish to receive, you specify a matching routing key.
// For each field of the routing key, you can either match against a specific
// value, or match all entries with the '*' wildcard. Above, we specified the
// following routing key and exchange:
//
//  		pulse.Bind( // routing key and exchange to get messages from
//  		"*.*.*.*.*.*.gaia.#",
//  		"exchange/taskcluster-queue/v1/task-defined"),
//
// This would match all messages on the exchange
// "exchange/taskcluster-queue/v1/task-defined" which have a workerType of
// "gaia" (see the taskDefined link above).  Notice also the '#' at the end of
// the string. This means "match all remaining fields" and can be used to match
// whatever comes after.
//
// To see the list of available exchanges on pulse, visit
// https://wiki.mozilla.org/Auto-tools/Projects/Pulse/Exchanges.
//
// After deciding which exchanges you are interested in, you need a queue to
// have them copied onto. This is also handled by the Consume method, with the
// first argument being the name of the queue to use.  You will notice above
// there are two types of queues we create: named queues, and unnamed queues:
//
//  	conn.Consume(
//  		"taskprocessing", // queue name
//
//  	conn.Consume( // a second workflow to manage concurrently
//  		"", // empty name implies anonymous queue
//
// To understand the difference, first we need to explain the different
// scenarios in which you might want to use them.
//
// Scenario 1) You have one client reading from the queue, and when you
// disconnect, you don't want your queue to receive any more messages
//
// Scenario 2) you have multiple clients that want to feed from the same queue
// (e.g.  when multiple workers can perform the same task, and whichever one
// pops the message off the queue first should process it)
//
// Scenario 3) you only have a single client reading from the queue, but if you
// go offline (crash, network interrupts etc) then you want pulse to keep
// updating your queue so your missed messages are there when you get back.
//
// In scenario 1 above, your client only uses the queue for the scope of the
// connection, and as soon as it disconnects, does not require the queue any
// further. In this case, an unnamed queue can be created, by passing "" as the
// queue name. When the connection closes, the AMQP server will automatically
// delete the queue.
//
// In scenarios 2 it is useful to have a friendly name for the queue that can
// be shared by all the clients using it.  The queue also should not be deleted
// when one client disconnects, it needs to live indefinitely. By providing a
// name for the queue, this signifies to the pulse library, that the queue
// should persist after a disconnect, and pulse should continue to populate the
// queue, even if no pulse clients are connected to consume the messages.
// Please note eventually the Pulse Guardian will delete your queue if you
// leave it collecting messages without consuming them.
//
// Scenario 3 is essentially the same as scenario 2 but with one consumer only.
// Again, a named queue is required.
//
// So, we're nearly done now. We now have a means to consume messages, by
// calling the Consume method, and specifying a queue name, some bindings of
// exchanges and routing keys, but how to actually process messages arriving on
// the queue?
//
// You will notice the Consume method takes a callback function. This can be an
// inline function, or point to any available function in your go code. You
// simply need to have a function that accepts an amqp.Delivery input, and pass
// it into the Consume method. Above, we did it like this:
//
//  		func(message interface{}, delivery amqp.Delivery) { // callback function to pass messages to
//  			fmt.Println("Received from exchange " + delivery.Exchange + ":")
//  			fmt.Println(string(delivery.Body))
//  			fmt.Println("")
//  			delivery.Ack(false) // acknowledge message *after* processing
//  		},
//
// The two parameters of the callback function we have created are the message
// object, and the delivery object. The message object is the pulse message,
// but unmarshaled into an interface{}. Since the pulse messages are all json
// messages, the pulse library unmarshals it and give you back a go object with
// its contents. Please note if you require that the json is unmarshaled into
// something more specific than interface{}, such as a custom class, this is
// possible, and will be explained in the next paragraph.  The other parameter,
// the delivery object, is an underlying amqp library type, which gives you
// access to some meta data for the message.  Please see
// http://godoc.org/github.com/streadway/amqp#Delivery for more information.
// Among other things, it provides you with delivery.Body, which is the raw
// json of the message. You can therefore choose if you want to process the raw
// json or the unmarshaled json in your callback method.
//
// You recall above that to describe the binding from an exchange to a queue
// with a given routing key, we specified pulse.Bind(routingKey, exchange) as a
// parameter of the Consume method. pulse.Bind(routingKey, exchange) returns an
// object of type Binding, where Binding is an interface.  If you wish to
// unmarshal your json into something other than an interface{}, take a look at
// the Binding interface documentation
// (http://godoc.org/github.com/taskcluster/pulse-go/pulse#Binding). Instead of
// calling pulse.Bind(routingKey, exchange) you can provide your own Binding
// interface implementation which can enable custom handling of exchange names,
// routing keys, and unmarshaling of objects. The taskcluster go client relies
// heavily on this, for example. See
// http://godoc.org/github.com/taskcluster/taskcluster-client-go/queueevents#example-package--TaskclusterSniffer
// for inspiration.
//
// In this example above, we simply output the information we receive, and then
// acknowledge receipt of the message. But why do we need to do this? To explain,
// take a look at the remaining parameters to Consume that we pass in. There
// are two more we have not discussed yet: they are the prefetch size (how many
// messages to fetch at once), and a bool to say whether to auto-acknowledge
// messages or not.
//
//  		1,     // prefetch 1 message at a time
//  		false, // don't auto-acknowledge messages
//
// When you acknowledge a message, it gets popped off the queue. If you don't
// auto-acknowledge, and also don't manually acknowledge, your queue is going
// to grow until it gets deleted by Pulse Guardian, so better to acknowledge
// those messages!  Auto-acknowledge happens when you receive the message; if
// you crash after receiving it but before processing it, you may have a
// problem. If it is important not to lose messages in such a scenario, you can
// acknowledge manually *after* processing the message. See above:
//
//  			delivery.Ack(false) // acknowledge message *after* processing
//
// This is "more work" for you to do, but guarantees that you don't lose
// messages. To handle situation of crashing after processing, but before
// acknowledging, having an idempotent message processing function (the
// callback) should help avoid the problem of processing a message twice.
//
// Please note the Consume method will take care of connecting to the pulse
// server (if no connection has yet been established), creating an AMQP
// channel, creating or connecting to an existing queue, binding it to all the
// exchanges and routing keys that you specify, and spawning a dedicated go
// routine to process the messages from this queue and feed them back to the
// callback method you provide.
//
// The client is implemented in such a way that a new AMQP channel is created
// for each queue that you consume, and that a separate go routine handles
// calling the callback function you specify. This means you can take advantage
// of go's built in concurrency support, and call the Consume method as many
// times as you wish.
//
// The aim of this library is to shield users from this lower-level resource
// management, and provide a simple interface in order to quickly and easily
// develop components that can interact with pulse.
package pulse
