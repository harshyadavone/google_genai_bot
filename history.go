package main

import (
	"github.com/google/generative-ai-go/genai"
	"sync"
)

// Conversation represents a single turn in the conversation.
type Conversation struct {
	Role  string       `json:"role"`
	Parts []genai.Part `json:"parts"`
}

// ChatHistory holds the conversation history for a specific chatId.
type ChatHistory struct {
	ChatID  int            `json:"chatId"`
	History []Conversation `json:"history"`
	mu      sync.Mutex
}

// Global map to store chat histories, keyed by chatId.
var chatHistories = make(map[int]*ChatHistory)

// AddMessage appends a new message (user or AI) to the conversation history.
func (ch *ChatHistory) AddMessage(role string, parts ...genai.Part) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.History = append(ch.History, Conversation{
		Role:  role,
		Parts: parts,
	})
}

// AddFunctionCall adds a function call to the conversation history.
func (ch *ChatHistory) AddFunctionCall(call *genai.FunctionCall) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.History = append(ch.History, Conversation{
		Role:  "model", // Function calls are from the model
		Parts: []genai.Part{call},
	})
}

// AddFunctionResponse adds a function response to the conversation history.
func (ch *ChatHistory) AddFunctionResponse(response *genai.FunctionResponse) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.History = append(ch.History, Conversation{
		Role:  "function", // ? Function responses have the role "function"
		Parts: []genai.Part{response},
	})
}

// GetLastNMessages retrieves the last n messages from the conversation history.
func (ch *ChatHistory) GetLastNMessages(n int) ([]*genai.Content, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	lenHistory := len(ch.History)
	startIndex := 0
	if lenHistory > n {
		startIndex = lenHistory - n
	}

	var messages []*genai.Content
	for i := startIndex; i < lenHistory; i++ {
		messages = append(messages, &genai.Content{
			Parts: ch.History[i].Parts,
			Role:  ch.History[i].Role,
		})
	}

	return messages, nil
}

// getOrCreateChatHistory retrieves the ChatHistory for a given chatId.
// If the chatId doesn't exist, it creates a new, empty ChatHistory.
func getOrCreateChatHistory(chatId int) *ChatHistory {
	history, ok := chatHistories[chatId]
	if !ok {
		history = &ChatHistory{
			ChatID:  chatId,
			History: []Conversation{},
		}
		chatHistories[chatId] = history
	}
	return history
}
