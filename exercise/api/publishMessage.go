package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PublishMessageInput struct {
	Id string `json:"id"`
}

type SpiderMessage struct {
	Id string
}

type Payload struct {
	Id string `json:"id"`
}

func publishMessageHandler(c *fiber.Ctx) error {
	publishMessageInput := new(PublishMessageInput)

	if err := c.BodyParser(publishMessageInput); err != nil {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: Cannot parse request to the struct, " + err.Error(),
		}
	}

	// Validate message
	if err := validatePublishMessageInput(*publishMessageInput); err != nil {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: invalid input, " + err.Error(),
		}
	}

	// Publish to the queue
	uri := os.Getenv("RABBITMQ_URI")
	queueName := "spider:msg"
	spiderMessage := SpiderMessage(*publishMessageInput)

	if err := publishMessage(uri, queueName, spiderMessage); err != nil {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: Cannot publish message to the transformer, " + err.Error(),
		}
	}

	// Return success response
	return c.JSON(fiber.Map{
		"status": "success",
	})
}

func validatePublishMessageInput(publicMessageInput PublishMessageInput) error {
	// error := errors.New("invalid input")

	return nil
}

// Context คืออะไรและใช้งานอย่างไรและควรใช้เมื่อไหร่, Present: Copy code ที่มีการใช้ Context มาเพื่อให้ทำงานได้ก่อน
func publishMessage(uri string, queueName string, message SpiderMessage) error {
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

	body, err := json.Marshal(Payload(message))
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
