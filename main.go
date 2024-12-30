package main

import (
	"google_genai/genai"
	"google_genai/telegram"
	"log"
	"net/http"
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

	bot := telegram.NewBot(os.Getenv("BOT_TOKEN"))

	genAIHandler := genai.NewHandler(bot)

	err := bot.SetWebhook(os.Getenv("WEBHOOK_URL"))
	if err != nil {
		log.Fatal("Error setting webhook:", err)
	}

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		update, err := bot.ParseUpdate(r)
		if err != nil {
			log.Printf("Error parsing update: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if update.Message != nil {
			if len(update.Message.Entities) > 0 && update.Message.Entities[0].Type == "bot_command" {
				bot.HandleCommands(update.Message.Chat.ID, update.Message.Text)
			} else {
				chatID := update.Message.Chat.ID
				text := update.Message.Text

				log.Printf("ChatId: %d \nText: %s", chatID, text)

				updateMessage, err := bot.SendLoadingMessage(chatID, "⏳")

				if err != nil {
					log.Println("Error sending loading message:", err)
				}

				go genAIHandler.HandleMessage(text, chatID, updateMessage)
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cleanup := genai.NewCleanupService("synapse_files")
	cleanup.Start()
	defer cleanup.Stop()

	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
