package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Println("No .env file found")
		os.Exit(1)
	}

	app := fiber.New()

	// Order sensitive
	app.Get("/", healthCheckHandler)
	app.Post("/pub", publishMessageHandler)

	app.Use(authMiddleware)
	app.Get("/get", getMessagesHandler)

	app.Listen(":3000")
}

type Header struct {
	Secret string `json:"Secret"`
}

func authMiddleware(c *fiber.Ctx) error {
	header := new(Header)
	if err := c.ReqHeaderParser(header); err != nil {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: Cannot parse header",
		}
	}

	if header.Secret != "1234" {
		return &fiber.Error{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Error: You do not have access to this route",
		}
	}

	return c.Next()
}
