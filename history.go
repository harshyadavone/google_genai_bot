package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
)

type Conversation struct {
	Role  string       `json:"role"`
	Parts []genai.Part `json:"parts"`
}

type ChatHistory struct {
	ChatID     int       `json:"chat_id"`
	TimeStamps time.Time `json:"time_stamps"`

	mu      sync.Mutex
	History []Conversation `json:"history"`
}

var (
	chatHistories  sync.Map
	maxHistorySize = 15
)

func (ch *ChatHistory) trimHistory() {
	if len(ch.History) > maxHistorySize {
		newHistory := make([]Conversation, maxHistorySize)
		copy(newHistory, ch.History[len(ch.History)-maxHistorySize:])
		ch.History = newHistory
	}
}

func (ch *ChatHistory) AddMessage(role string, parts ...genai.Part) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.History = append(ch.History, Conversation{
		Role:  role,
		Parts: parts,
	})

	ch.trimHistory()
}

func (ch *ChatHistory) AddFunctionCall(call *genai.FunctionCall) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.History = append(ch.History, Conversation{
		Role:  "model",
		Parts: []genai.Part{call},
	})

	ch.trimHistory()
}

func (ch *ChatHistory) AddFunctionResponse(response *genai.FunctionResponse) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.History = append(ch.History, Conversation{
		Role:  "function",
		Parts: []genai.Part{response},
	})

	ch.trimHistory()
}

func (ch *ChatHistory) GetLastMessages() ([]*genai.Content, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if len(ch.History) == 0 {
		return nil, fmt.Errorf("no message available")
	}

	// var messages []*genai.Content
	messages := make([]*genai.Content, 0, len(ch.History))
	for _, v := range ch.History {
		messages = append(messages, &genai.Content{
			Parts: v.Parts,
			Role:  v.Role,
		})
	}

	return messages, nil
}

func getOrCreateChatHistory(chatId int) *ChatHistory {

	if history, ok := chatHistories.Load(chatId); ok {
		return history.(*ChatHistory)
	}

	newHistory := &ChatHistory{
		ChatID:  chatId,
		History: []Conversation{},
	}

	actual, loaded := chatHistories.LoadOrStore(chatId, newHistory)
	if loaded {
		return actual.(*ChatHistory)
	}

	return newHistory
}
