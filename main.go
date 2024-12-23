package main

import (
	"log"
	"os"
)

func main() {
	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	if os.Getenv("BOT_TOKEN") == "" {
		log.Fatal("BOT_TOKEN environment variable is not set")
	}

	if os.Getenv("WEBHOOK_URL") == "" {
		log.Fatal("WEBHOOK_URL environment variable is not set")
	}

	if os.Getenv("PORT") == "" {
		log.Fatal("WEBHOOK_URL environment variable is not set")
	}

	InitTelegramBot()
}
