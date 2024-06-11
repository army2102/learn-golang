package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Payload struct {
	Id string `json:"id"`
}

type TrendMessage struct {
	Id string
}

// เมื่อไหร่ควร Pass by value เมื่อไหร่ควร Pass by reference, Present: Pass by value กับทุกอย่างยกเว้นเป็น Reference Type อยู่แล้ว
func main() {
	// Get config
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Error: No .env file found")
		os.Exit(1)
	}

	rabbitMqUri := os.Getenv("RABBITMQ_URI")

	// Consume payload from the queue
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, payload amqp.Delivery) {
		// Transform the payload into Trend message
		data := Payload{}
		if err := json.Unmarshal(payload.Body, &data); err != nil {
			log.Panic("Cannot parse payload to spider message" + err.Error())
		}
		log.Println("Receive payload: ", data)

		trendMessage, err := transformToTrendMessage(data)
		if err != nil {
			log.Panic("Cannot transform payload to Trend message" + err.Error())
		}

		// Publish Trend message to the bulk worker
		queueName := "bulk:msg"
		if err := publish(rabbitMqUri, queueName, trendMessage); err != nil {
			log.Panic("Cannot parse payload to spider message" + err.Error())
		}

	}

	queueName := "spider:msg"
	consume(ctx, rabbitMqUri, queueName, handler)

}

func consume(ctx context.Context, uri string, queueName string, handler func(ctx context.Context, payload amqp.Delivery)) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		log.Panic("cannot connect to rabbitmq, " + err.Error())
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Panic("cannot connect to rabbitmq channel, " + err.Error())
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Panic("cannot create queue, " + err.Error())
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Panic("cannot consume payload from the queue, " + err.Error())
	}

	var forever chan struct{}

	go func() {
		for d := range msgs {
			handler(ctx, d)
		}
	}()

	log.Printf("waiting for messages, press ctrl+c to exit")
	<-forever
}

func publish(uri string, queueName string, trendMessage TrendMessage) error {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return errors.New("cannot connect to rabbitmq, " + err.Error())
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return errors.New("cannot connect to rabbitmq channel, " + err.Error())
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return errors.New("cannot create queue, " + err.Error())
	}

	body, err := json.Marshal(Payload(trendMessage))
	if err != nil {
		return errors.New("cannot parse message to json, " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = ch.PublishWithContext(ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediateg
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		return errors.New("cannot publish to the queue, " + err.Error())
	}

	return nil
}

// TODO: Define a Spider message struct & Trend struct
// TODO: Transform
func transformToTrendMessage(spiderMessagePayload Payload) (TrendMessage, error) {
	trendMessage := TrendMessage(spiderMessagePayload)

	return trendMessage, nil
}
