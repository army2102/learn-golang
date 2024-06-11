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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Payload struct {
	Id string `json:"id"`
}

type TrendMessage struct {
	Id string
}

type TrendMessageMongo struct {
	Id   primitive.ObjectID `bson:"_id"`
	Data string             `bson:"data"`
}

// เมื่อไหร่ควร Pass by value เมื่อไหร่ควร Pass by reference, Present: Pass by value กับทุกอย่างยกเว้นเป็น Reference Type อยู่แล้ว
func main() {
	// Get config
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Error: No .env file found")
		os.Exit(1)
	}

	// Consume payload from the queue
	rabbitMqUri := os.Getenv("RABBITMQ_URI")
	queueName := "bulk:msg"
	trendMessages := []TrendMessage{}
	handler := func(ctx context.Context, payload <-chan amqp.Delivery) {
		mongoDbUri := os.Getenv("MONGODB_URI")
		dbName := "golang_exercise"
		collectionName := "messages"

		for {
			select {
			case d := <-payload:
				// Transform the payload into Trend message
				data := Payload{}
				if err := json.Unmarshal(d.Body, &data); err != nil {
					log.Panic("Cannot parse payload to spider message" + err.Error())
				}
				log.Println("Receive payload: ", data)

				trendMessages = append(trendMessages, TrendMessage(data))
				log.Println("Bulk len: ", len(trendMessages))

				// Save the message into the database if batch size is reach
				bulkSize := 10
				if len(trendMessages) >= bulkSize {
					log.Println("Bulk size reached, save!")
					if err := bulkInsert(mongoDbUri, dbName, collectionName, trendMessages); err != nil {
						log.Panic("Cannot transform payload to Trend message" + err.Error())
					}
					trendMessages = []TrendMessage{}
					log.Println("Reset bulk")
				}

			case <-ctx.Done():
				// Save the message into the database
				log.Println("Timeout reached, save!")
				if err := bulkInsert(mongoDbUri, dbName, collectionName, trendMessages); err != nil {
					log.Panic("Cannot transform payload to Trend message" + err.Error())
				}
				trendMessages = []TrendMessage{}

				// Reset the context
				return
			}
		}

	}

	handlerTimeout := time.Second * 10
	consume(rabbitMqUri, queueName, handler, handlerTimeout)

}

func consume(uri string, queueName string, handler func(ctx context.Context, payload <-chan amqp.Delivery), handlerTimeout time.Duration) {
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
		for {
			ctxTimeout, cancel := context.WithTimeout(context.TODO(), handlerTimeout)

			handler(ctxTimeout, msgs)

			cancel()
			log.Println("Timeout reached, restarting timeout period")
		}
	}()

	log.Printf("waiting for messages, press ctrl+c to exit")
	<-forever
}

// TODO: Save to the database
func bulkInsert(uri string, dbName string, collectionName string, trendMessages []TrendMessage) error {
	log.Println("Save Trend message: ", trendMessages)

	// Validate uri (config)
	if uri == "" {
		return errors.New("empty uri")
	}

	// Connect to the database
	client, err := mongo.Connect(context.TODO(), options.Client().
		ApplyURI(uri))

	if err != nil {
		return errors.New("cannot connect to the database" + err.Error())
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database(dbName).Collection(collectionName)

	docs := make([]interface{}, len(trendMessages))
	for index, message := range trendMessages {
		docs[index] = TrendMessageMongo{
			Id:   primitive.NewObjectID(),
			Data: message.Id,
		}
	}

	log.Printf("%+v", docs)

	coll.InsertMany(context.TODO(), docs)

	return nil
}
