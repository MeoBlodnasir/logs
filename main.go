package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/natefinch/pie"
	"github.com/streadway/amqp"
	"net/rpc/jsonrpc"
	"os"
)

var (
	name = "logs"
	srv  pie.Server
)

// Plugin Structure
type api struct{}

type LogInfos struct {
	Action  string
	Who     string
	What    string
	Module  string
	Success bool
	Msg     string
}

func handleLogs(msg LogInfos) {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.WithFields(log.Fields{
		"Who":     msg.Who,
		"Module":  msg.Module,
		"Action":  msg.Action,
		"What":    msg.What,
		"Success": msg.Success,
		"Msg":     msg.Msg,
	}).Info("")
}

// Continuously look for stuff to log in the log queue of RabbitMQ
func launchlogs() {
	initConf()
	conn, err := amqp.Dial(conf.QueueUri)
	if err != nil {
		log.Error("Failed to connect to RabbitMQ server: ", err)
	}
	ch, err := conn.Channel()
	defer ch.Close()
	defer conn.Close()
	if err != nil {
		log.Error("Failed to open a RabbitMQ channel: ", err)
	}

	err = ch.ExchangeDeclare(
		"users_topic", // name
		"topic",       // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		log.Error("Failed to declare a RabbitMQ exchange: ", err)
	}

	_, err = ch.QueueDeclare(
		"logs", // name
		true,   // durable
		false,  // delete when usused
		false,  // exclusive
		false,  // no-wait
		nil,    // arguments
	)
	if err != nil {
		log.Error("Failed to declare a RabbitMQ queue: ", err)
	}

	err = ch.QueueBind(
		"logs",        // queue name
		"logs",        // routing key
		"users_topic", // exchange
		false,
		nil)
	if err != nil {
		log.Error("Failed to bind a RabbitMQ queue: ", err)
	}
	msgs, err := ch.Consume(
		"logs", // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Error("Failed to declare a RabbitMQ consummer: ", err)
	}

	forever := make(chan bool)

	go func() {
		var msg LogInfos
		for d := range msgs {
			err := json.Unmarshal(d.Body, &msg)
			if err != nil {
				log.Error("Failed to Unmarshal the message received: ", err)
			}
			handleLogs(msg)

		}
	}()
	<-forever
}

// Plug the plugin to the core
func (api) Plug(args interface{}, reply *bool) error {
	go launchlogs()
	*reply = true
	return nil
}

// Will contain various verifications for the plugin. If the core can call the function and receives "true" in the reply, it means the plugin is functionning correctly
func (api) Check(args interface{}, reply *bool) error {
	*reply = true
	return nil
}

// Unplug the plugin from the core
func (api) Unplug(args interface{}, reply *bool) error {
	defer os.Exit(0)
	*reply = true
	return nil
}

func main() {
	srv = pie.NewProvider()
	if err := srv.RegisterName(name, api{}); err != nil {
		log.Fatal("Failed to register:", name, err)
	}
	srv.ServeCodec(jsonrpc.NewServerCodec)
}
