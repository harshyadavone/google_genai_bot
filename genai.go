package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

func handleGenAI(userMessage string, chatID int) {

	ctx := context.Background()

	ctx = context.WithValue(ctx, "chatId", chatID)

	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-exp")
	model.SystemInstruction = genai.NewUserContent(genai.Text(InitialSystemPrompt))
	model.Tools = []*genai.Tool{tools}
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
	}

	chatHistory := getOrCreateChatHistory(chatID)

	cs := model.StartChat()

	chatHistory.AddMessage("user", genai.Text(userMessage))

	// Last n (15) messages for context
	lastMessages, err := chatHistory.GetLastNMessages(15)
	if err != nil {
		log.Println("Error getting last messages:", err)
	}

	cs.History = lastMessages

	res, err := cs.SendMessage(ctx, genai.Text(userMessage))

	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}

	handleResponse(ctx, cs, res, chatID)
}

func handleResponse(ctx context.Context, cs *genai.ChatSession, resp *genai.GenerateContentResponse, userId int) {
	if resp == nil {
		return
	}

	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}

		for _, part := range cand.Content.Parts {
			switch v := part.(type) {
			case genai.Text:
				if text := strings.TrimSpace(string(v)); text != "" {
					fmt.Printf("1. Gemini: %s\n", text)

					history := getOrCreateChatHistory(userId)

					history.AddMessage("model", v)

					if err := sendMessage(userId, text); err != nil {
						log.Println("Error sending message to Telegram:", err)
					}
				}

			case genai.FunctionCall:
				history := getOrCreateChatHistory(userId)

				toolFunc, err := getTool(v.Name)
				if err != nil {
					log.Printf("Error retrieving tool: %v\n", err)
					sendToolError(ctx, cs, v.Name, fmt.Sprintf("Tool '%s' not found.", v.Name), userId)
					continue
				}

				history.AddFunctionCall(&v)

				result, err := toolFunc(ctx, v)
				if err != nil {
					log.Printf("Error executing tool '%s': %v\n", v.Name, err)
					sendToolError(ctx, cs, v.Name, err.Error(), userId)
					continue
				}

				fmt.Println("Function executed successfully:", result)

				// WARN: update it...
				if strings.HasPrefix(result, "File created successfully at") {
					filePath := strings.TrimPrefix(result, "File created successfully at ")
					err = sendFileWithProgress(userId, filePath)
					if err != nil {
						log.Printf("Error sending file: %v\n", err)
					}
				}

				nextResp, err := cs.SendMessage(ctx, genai.FunctionResponse{
					Name:     v.Name,
					Response: map[string]any{"function response: ": result},
				})

				history.AddFunctionResponse(&genai.FunctionResponse{
					Name:     v.Name,
					Response: map[string]any{"function response: ": result},
				})

				if err != nil {
					log.Printf("Error sending function response first response's nextResp: %v", err)
					continue
				}

				if hasNonEmptyContent(nextResp) {
					handleResponse(ctx, cs, nextResp, userId)
				}

			default:
				fmt.Printf("Gemini: (Non-textual response) %v\n", part)
			}
		}
	}
}

func sendToolError(ctx context.Context, cs *genai.ChatSession, toolName, errorMsg string, userId int) {
	resp, err := cs.SendMessage(ctx, genai.FunctionResponse{
		Name: toolName,
		Response: map[string]any{
			"error": errorMsg,
		},
	})

	history := getOrCreateChatHistory(userId)

	history.AddFunctionResponse(&genai.FunctionResponse{
		Name: toolName,
		Response: map[string]any{
			"error": errorMsg,
		},
	})

	if err != nil {
		log.Printf("Error sending error response: %v", err)
		return
	}

	handleResponse(ctx, cs, resp, userId)
}

// Print the response
func printResponse(resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
			}
		}
	}
}

func hasNonEmptyContent(resp *genai.GenerateContentResponse) bool {
	if resp == nil {
		return false
	}

	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}

		for _, part := range cand.Content.Parts {
			switch v := part.(type) {
			case genai.Text:
				if text := strings.TrimSpace(string(v)); text != "" {
					return true
				}
			case genai.FunctionCall:
				return true
			default:
				if v != nil {
					return true
				}
			}
		}
	}
	return false
}
