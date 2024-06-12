package main

import (
	"context"
	"errors"
	"math"
	"os"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type GetMessageInput struct {
	Page     int `query:"offset"`
	PageSize int `query:"limit"`
}

type GetMessagesOutput MessagePagination

type MessagePagination struct {
	Data            []Message
	CurrentPage     int
	CurrentPageSize int
	TotalPages      int
	TotalItems      int
}

type Message struct {
	Id string
}

type TrendMessageMongo struct {
	Id primitive.ObjectID `bson:"_id"`
}

func getMessagesHandler(c *fiber.Ctx) error {
	getMessagesInput := new(GetMessageInput)

	if err := c.QueryParser(getMessagesInput); err != nil {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: Cannot parse request to the struct",
		}
	}

	// Validate query strings
	err := validateGetMessagesInput(*getMessagesInput)
	if err != nil {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: invalid input",
		}
	}

	// Get messages from the database based on page and pageSize
	uri := os.Getenv("MONGODB_URI")
	messagePagination, err := getMessages(uri, getMessagesInput.Page, getMessagesInput.PageSize)
	if err != nil {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: Cannot get messages",
		}
	}

	// return as response
	return c.JSON(GetMessagesOutput(messagePagination))
}

func validateGetMessagesInput(input GetMessageInput) error {
	// err := errors.New("an error occurred")

	return nil
}

// Error handling ตาม Best Practice ควรทำอย่างไร, Present: Return ออกไปให้ข้างนอก Manage
// bson.D ต่างกับ bson.M อย่างไร, Present: ใช้ bson.M อยู่เพราะหน้าตาคล้ายใน TypeScript
// รู้สึกว่ามี Primative Type ที่มีหลาย Variation ควรมีการเลือกใช้อย่างไร, Present: Function อยากได้อะไรก็ Cast เป็นอันนั้นให้
// Context คืออะไร ใช้งานอย่างไร ใช้เมื่อไหร่, Present: ใส่ไปก่อนเพื่อให้ Code run ผ่าน
func getMessages(uri string, page int, pageSize int) (MessagePagination, error) {
	// Validate uri (config)
	if uri == "" {
		err := errors.New("invalid uri")

		return MessagePagination{}, err
	}

	// Connect to the database
	client, err := mongo.Connect(context.TODO(), options.Client().
		ApplyURI(uri))

	if err != nil {
		return MessagePagination{}, err
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("golang_exercise").Collection("messages")

	// Get raw messages from the database
	if page < 1 {
		err := errors.New("page cannot be less then 1")
		return MessagePagination{}, err
	}

	filter := bson.M{}
	totalItems, err := coll.CountDocuments(context.TODO(), filter)
	if err != nil {
		return MessagePagination{}, err
	}
	if totalItems == 0 {
		return MessagePagination{}, nil
	}

	totalPages := math.Ceil(float64(totalItems) / float64(pageSize))

	// Adjust page number if it's greater than the total pages
	searchPage := page
	if page > int(totalPages) {
		searchPage = int(totalPages)
	}

	findOptions := options.Find()
	findOptions.SetSkip(int64((searchPage - 1) * pageSize))
	findOptions.SetLimit(int64(pageSize))

	cursor, err := coll.Find(context.TODO(), filter, findOptions)
	if err != nil {
		return MessagePagination{}, err
	}

	defer cursor.Close(context.TODO())

	// Convert raw messages to intended messages
	var messages []Message
	for cursor.Next(context.TODO()) {
		var messageMongo TrendMessageMongo

		err := cursor.Decode(&messageMongo)

		if err != nil {
			return MessagePagination{}, err
		}

		message := Message{
			Id: messageMongo.Id.Hex(),
		}
		messages = append(messages, message)
	}

	if err := cursor.Err(); err != nil {
		return MessagePagination{}, err
	}

	messagesPagination := MessagePagination{
		Data:            messages,
		CurrentPage:     page,
		CurrentPageSize: pageSize,
		TotalPages:      int(totalPages),
		TotalItems:      int(totalItems),
	}

	return messagesPagination, nil
}
