package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var telegramAPI string

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      User   `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type User struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

type SendMessageRequest struct {
	ChatID    int    `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

type SendMessageResponse struct {
	Ok bool `json:"ok"`
	Result
}

type Result struct {
	MessageID int `json:"message_id"`
}

func InitTelegramBot() {

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN environment variable is not set")
	}
	telegramAPI = "https://api.telegram.org/bot" + botToken

	webhookURL := os.Getenv("WEBHOOK_URL")
	err := setWebhook(webhookURL)
	if err != nil {
		log.Fatal("Error setting webhook: ", err)
	}

	http.HandleFunc("/webhook", handleWebhook)

	Port := os.Getenv("PORT")

	log.Printf("Starting the server on port %s", Port)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading request:", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	log.Printf("Request BODY: %s", string(body))
	defer r.Body.Close()

	var update Update
	err = json.Unmarshal(body, &update)
	if err != nil {
		log.Println("Error unmarshaling JSON:", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if update.Message != nil {
		chatID := update.Message.Chat.ID
		text := update.Message.Text
		handleGenAI(text, chatID)
	}
	w.WriteHeader(http.StatusOK)
}

func sendMessage(chatID int, text string) error {
	// escapedText := EscapeMarkdownV2(text)
	htmlText := convertToTelegramHTML(text)

	reqBody := SendMessageRequest{
		ChatID:    chatID,
		Text:      htmlText,
		ParseMode: "HTML",
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	resp, err := http.Post(telegramAPI+"/sendMessage", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body)
	}
	return nil
}

func setWebhook(webhookURL string) error {
	url := fmt.Sprintf("%s/setWebhook?url=%s", telegramAPI, webhookURL)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set webhook: %s", body)
	}
	return nil
}

func sendLoadingMessage(chatID int64, text string) (func(), error) {
	msg, err := sendMessageAndGetID(chatID, text)
	if err != nil {
		return nil, err
	}

	return func() {
		err := deleteMessage(chatID, msg.MessageID)
		if err != nil {
			log.Printf("Error updating message: %v", err)
		}
	}, nil
}

func sendMessageAndGetID(chatID int64, text string) (*Message, error) {
	url := fmt.Sprintf("%s/sendMessage", telegramAPI)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Ok     bool    `json:"ok"`
		Result Message `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Result, nil
}

func deleteMessage(chatID int64, messageID int) error {
	url := fmt.Sprintf("%s/deleteMessage", telegramAPI)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return nil
}

func sendDocument(chatID int, filePath string) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("error creating form file: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("error copying file content: %v", err)
	}

	err = writer.WriteField("chat_id", strconv.Itoa(chatID))
	if err != nil {
		return fmt.Errorf("error writing chat_id field: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("error closing writer: %v", err)
	}

	url := fmt.Sprintf("%s/sendDocument", telegramAPI)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func sendFileWithProgress(chatID int, filePath string) error {
	if filePath == "" {
		return fmt.Errorf("invalid file path")
	}

	msg, err := sendMessageAndGetID(int64(chatID), "Preparing your file...")
	if err != nil {
		return fmt.Errorf("error sending initial message: %v", err)
	}

	err = updateMessage(int64(chatID), msg.MessageID, "Uploading file...")
	if err != nil {
		log.Printf("Error updating progress message: %v", err)
	}

	err = sendDocument(chatID, filePath)
	if err != nil {
		updateMessage(int64(chatID), msg.MessageID, "Error sending file!")
		return fmt.Errorf("error sending document: %v", err)
	}

	err = deleteMessage(int64(chatID), msg.MessageID)
	if err != nil {
		log.Printf("Error deleting progress message: %v", err)
	}

	return nil
}

func updateMessage(chatID int64, messageID int, text string) error {
	url := fmt.Sprintf("%s/editMessageText", telegramAPI)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return nil
}

var (
	patterns = []struct {
		regex *regexp.Regexp
		html  interface{}
	}{
		{regexp.MustCompile("(?s)```([a-zA-Z]*)\n(.*?)\n```"), nil}, // Will be initialized with function
		{regexp.MustCompile("`([^`]+)`"), "<code>$1</code>"},
		{regexp.MustCompile(`\*\*(.*?)\*\*`), "<b>$1</b>"},
		{regexp.MustCompile(`__(.*?)__`), "<u>$1</u>"},
		{regexp.MustCompile(`_(.*?)_`), "<i>$1</i>"},
		{regexp.MustCompile(`~~(.*?)~~`), "<s>$1</s>"},
		{regexp.MustCompile(`\|\|(.*?)\|\|`), "<tg-spoiler>$1</tg-spoiler>"},
		{regexp.MustCompile(`\[(.*?)\]\((.*?)\)`), "<a href=\"$2\">$1</a>"},
		{regexp.MustCompile(`(?m)^\*\s+(.*)`), "• $1"}, // ?m -> makes `^` work per line not just start and end of the input
	}

	multipleNewlines = regexp.MustCompile(`\n{3,}`)
	lineSpaces       = regexp.MustCompile(`(?m)^[ \t]+|[ \t]+$`)

	builderPool = sync.Pool{
		New: func() interface{} {
			return new(strings.Builder)
		},
	}
)

func init() {
	patterns[0].html = func(matches []string) string {
		if len(matches) < 3 {
			return matches[0]
		}
		code := strings.TrimSpace(matches[2])
		return fmt.Sprintf("<pre><code>%s</code></pre>", code)
	}
}

func convertToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	defer builderPool.Put(builder)

	builder.Grow(len(text) * 2)

	text = escapeUserInputForHTML(text)

	for _, pattern := range patterns {
		switch replacement := pattern.html.(type) {
		case string:
			text = pattern.regex.ReplaceAllString(text, replacement)
		case func([]string) string:
			text = pattern.regex.ReplaceAllStringFunc(text, func(match string) string {
				groups := pattern.regex.FindStringSubmatch(match)
				return replacement(groups)
			})
		}
	}

	text = cleanupText(text)

	return text
}

func escapeUserInputForHTML(text string) string {
	builder := builderPool.Get().(*strings.Builder)
	builder.Reset()
	builder.Grow(len(text) + len(text)/4)
	defer builderPool.Put(builder)

	for _, r := range text {
		switch r {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func cleanupText(text string) string {
	text = multipleNewlines.ReplaceAllString(text, "\n\n")
	text = lineSpaces.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}
