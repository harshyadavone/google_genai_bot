package main

import (
	"log"
	"os"
)

func main() {
	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	InitTelegramBot()
}
