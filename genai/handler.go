package genai

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Handler struct {
	bot             TelegramBot
	processingState map[int]*ProcessingState
	stateMutex      sync.RWMutex
}

type ProcessingState struct {
	IsProcessing    bool
	StartTime       time.Time
	TimeoutDuration time.Duration
}

type TelegramBot interface {
	HandleSendMessage(chatID int, text string) error
	SendLoadingMessage(chatID int, text string) (int, error)
	// UpdateMessage(chatID int, messageID int, text string) error
	HandleUpdateMessage(chatID int, messageID int, text string) error
	SendFileWithProgress(chatID int, filepath string) error
}

type MessageWithID struct {
	Text      string
	MessageID int
}

func NewHandler(bot TelegramBot) *Handler {
	return &Handler{
		bot:             bot,
		stateMutex:      sync.RWMutex{},
		processingState: make(map[int]*ProcessingState),
	}
}

func (h *Handler) tryAcquireProcessing(chatId int) bool {
	h.stateMutex.Lock()
	defer h.stateMutex.Unlock()

	state, exists := h.processingState[chatId]
	if !exists {
		log.Printf("Creating new state for chat %d", chatId)

		state = &ProcessingState{
			TimeoutDuration: 2 * time.Minute,
			IsProcessing:    false,
		}
		h.processingState[chatId] = state
	}

	if state.IsProcessing {
		// if time.Since(state.StartTime) > state.TimeoutDuration {
		// 	state.IsProcessing = false
		// 	log.Printf("Chat %d timed out, allowing new processing", chatId)
		// 	// goto DONE
		// } else {
		log.Printf("Chat %d is busy", chatId)
		return false
		// }
	}

	// DONE:
	log.Printf("Starting processing for chat %d", chatId)
	state.IsProcessing = true
	state.StartTime = time.Now()
	return true
}

func (h *Handler) releaseProcessing(chatId int) {
	h.stateMutex.Lock()
	defer h.stateMutex.Unlock()

	if state, exists := h.processingState[chatId]; exists {
		state.IsProcessing = false
	}

}

func (h *Handler) startCleanupRoutine() {
	ticker := time.NewTicker(time.Minute * 5)
	for range ticker.C {
		h.cleanup()
	}
}

func (h *Handler) cleanup() {
	h.stateMutex.Lock()
	defer h.stateMutex.Unlock()

	for chatId, state := range h.processingState {
		if time.Since(state.StartTime) > state.TimeoutDuration*2 {
			delete(h.processingState, chatId)
		}
	}
}

func (h *Handler) ProcessMessage(userMessage string, chatID int, messageId int) {
	if !h.tryAcquireProcessing(chatID) {
		h.bot.HandleUpdateMessage(chatID, messageId, "Please wait, processing previous request...")
		return
	}
	defer h.releaseProcessing(chatID)

	defer func() {
		if r := recover(); r != nil {
			h.releaseProcessing(chatID)
			log.Printf("Recovered from panic in ProcessMessage: %v", r)
			h.bot.HandleUpdateMessage(chatID, messageId, "An error occurred, please try again")
		}
	}()

	ctx := context.Background()
	ctx = context.WithValue(ctx, "chatId", chatID)

	h.bot.HandleUpdateMessage(chatID, messageId, "⏳Processing your request...")

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
	lastMessages, err := chatHistory.GetLastMessages()
	if err != nil {
		log.Println("Error getting last messages:", err)
	}

	cs.History = lastMessages

	res, err := cs.SendMessage(ctx, genai.Text(userMessage))

	if err != nil {
		logWithTime("Error sending message: %v", err)
		h.bot.HandleUpdateMessage(chatID, messageId, "something went wrong!, please try again after sometime.")
		return
	}

	handleResponse(ctx, cs, h.bot, res, chatID, messageId, func() {
		h.releaseProcessing(chatID)
	})
}

func handleResponse(ctx context.Context, cs *genai.ChatSession, bot TelegramBot, resp *genai.GenerateContentResponse, chatId int, messageId int, onComplete func()) {
	defer onComplete()

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
					history := getOrCreateChatHistory(chatId)
					history.AddMessage("model", v)

					if len(text) < 4096 {
						bot.HandleUpdateMessage(chatId, messageId, text)
					} else {
						chunks := splitMessage(text, 4096)
						if len(chunks) > 0 {
							bot.HandleUpdateMessage(chatId, messageId, chunks[0])
						}

						for i := 1; i < len(chunks); i++ {
							if err := bot.HandleSendMessage(chatId, chunks[i]); err != nil {
								log.Printf("Error sending message chunk: %v", err)
							}
						}
					}

				}

			case genai.FunctionCall:
				history := getOrCreateChatHistory(chatId)

				toolFunc, err := getTool(v.Name)
				if err != nil {
					logWithTime("Error retrieving tool: %v\n", err)
					sendToolError(ctx, cs, bot, v.Name, fmt.Sprintf("Tool '%s' not found.", v.Name), chatId, messageId, onComplete)
					continue
				}

				history.AddFunctionCall(&v)

				bot.HandleUpdateMessage(chatId, messageId, fmt.Sprintf("Executing %s", v.Name))

				result, err := toolFunc(ctx, v)
				if err != nil {
					logWithTime("Error executing tool '%s': %v\n", v.Name, err)
					sendToolError(ctx, cs, bot, v.Name, err.Error(), chatId, messageId, onComplete)
					continue
				}

				logWithTime("%s Function executed successfully", v.Name)
				bot.HandleUpdateMessage(chatId, messageId, fmt.Sprintf("%s executed successfully", v.Name))

				// WARN: update it...
				if strings.HasPrefix(result, "File created successfully at") {
					filePath := strings.TrimPrefix(result, "File created successfully at ")
					err = bot.SendFileWithProgress(chatId, filePath)
					if err != nil {
						logWithTime("Error sending file: %v\n", err)
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
					logWithTime("Error sending function response first response's nextResp: %v", err)
					continue
				}

				if hasNonEmptyContent(nextResp) {
					handleResponse(ctx, cs, bot, nextResp, chatId, messageId, onComplete)
				}

			default:
				fmt.Printf("Gemini: (Non-textual response) %v\n", part)
			}
		}
	}
}

func sendToolError(ctx context.Context, cs *genai.ChatSession, bot TelegramBot, toolName, errorMsg string, chatId int, messageId int, onComplete func()) {
	resp, err := cs.SendMessage(ctx, genai.FunctionResponse{
		Name: toolName,
		Response: map[string]any{
			"error": errorMsg,
		},
	})

	bot.HandleUpdateMessage(chatId, messageId, errorMsg)
	history := getOrCreateChatHistory(chatId)

	history.AddFunctionResponse(&genai.FunctionResponse{
		Name: toolName,
		Response: map[string]any{
			"error": errorMsg,
		},
	})

	if err != nil {
		logWithTime("Error sending error response: %v", err)
		return
	}

	handleResponse(ctx, cs, bot, resp, chatId, messageId, onComplete)
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

func splitMessage(text string, maxLength int) []string {
	var chunks []string

	if len(text) > maxLength {
		chunk := text[:maxLength]

		lastSpace := strings.LastIndex(chunk, " ")

		if lastSpace == -1 {
			chunks = append(chunks, chunk)
			text = text[maxLength:]
		} else {
			chunks = append(chunks, text[:lastSpace])
			text = text[maxLength+1:]
		}
	}

	if len(chunks) > 0 {
		chunks = append(chunks, text)
	}
	return chunks
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
