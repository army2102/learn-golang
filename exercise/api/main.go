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

	app.Get("/", healthCheckHandler)

	app.Post("/pub", publishMessageHandler)
	app.Get("/get", getMessagesHandler)

	app.Listen(":3000")
}
