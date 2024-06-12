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

type BulkSaverPayload struct {
	Id        string `json:"id"`
	TrendData string `json:"trendData"`
}

type TrendMessage struct {
	Id        string
	TrendData string
}

type TrendMessageMongo struct {
	Id        primitive.ObjectID `bson:"_id"`
	TrendData string             `bson:"trendData"`
}

// เมื่อไหร่ควร Pass by value เมื่อไหร่ควร Pass by reference, Present: Pass by value กับทุกอย่างยกเว้นเป็น Reference Type อยู่แล้ว
func main() {
	// Get config
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Error: No .env file found")
		os.Exit(1)
	}

	// Get payload from the queue and put into a channel
	rabbitMqUri := os.Getenv("RABBITMQ_URI")
	queueName := "bulk:msg"
	messageChannel := make(chan amqp.Delivery)
	go consume(rabbitMqUri, queueName, messageChannel)

	trendMessages := []TrendMessage{}
	handler := func(ctx context.Context, messageChannel <-chan amqp.Delivery) {
		mongoDbUri := os.Getenv("MONGODB_URI")
		dbName := "golang_exercise"
		collectionName := "messages"

		for {
			select {
			case d := <-messageChannel:
				// Transform the payload into Trend message
				data := BulkSaverPayload{}
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

	// Handler receive message from the channel and processed it
	for {
		timeout := time.Second * 10
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		handler(ctx, messageChannel)

		cancel()
	}

}

func consume(uri string, queueName string, channel chan<- amqp.Delivery) {
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

	messageChannel, err := ch.Consume(
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
		for message := range messageChannel {
			channel <- message
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
			Id:        primitive.NewObjectID(),
			TrendData: message.TrendData,
		}
	}

	log.Printf("%+v", docs)

	coll.InsertMany(context.TODO(), docs)

	return nil
}
