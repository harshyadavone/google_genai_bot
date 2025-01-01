package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"google_genai/format"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type TelegramError struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

func (b *Bot) SendMessage(chatID int, text string) error {

	htmlText := format.ConvertToTelegramHTML(text)
	reqBody := SendMessageRequest{
		ChatID:    chatID,
		Text:      htmlText,
		ParseMode: "HTML",
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := http.Post(b.APIBaseURL+"/sendMessage", "application/json", bytes.NewBuffer(reqBytes))
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

func (b *Bot) SendLoadingMessage(chatID int, text string) (int, error) {
	msg, err := b.SendMessageAndGetID(chatID, text)
	if err != nil {
		return 0, err
	}

	return msg.MessageID, nil

	// return &msg.MessageID, func(messageId int, updatedText string) {
	// 	err := b.updateMessage(chatID, messageId, updatedText)
	// 	if err != nil {
	// 		log.Printf("Error updating message: %v", err)
	//
	// 		var teleErr *TelegramError
	// 		if errors.As(err, &teleErr) {
	// 			switch teleErr.Description {
	// 			case "Bad Request: MESSAGE_TOO_LONG":
	// 				_ = b.updateMessage(chatID, messageId,
	// 					"Sorry, the response was too long for Telegram. Please try again.")
	// 			case "Bad Request: can't parse entities":
	// 				plainErr := b.updateMessageWithoutHTML(chatID, messageId, updatedText)
	// 				if plainErr != nil {
	// 					_ = b.updateMessage(chatID, messageId,
	// 						"Sorry, I encountered an error while formatting the message. Please try again.")
	// 				}
	// 			default:
	// 				_ = b.updateMessage(chatID, messageId,
	// 					fmt.Sprintf("Error: %s", teleErr.Description))
	// 			}
	// 			return
	// 		}
	//
	// 		_ = b.updateMessage(chatID, messageId,
	// 			"An unexpected error occurred. Please try again.")
	// 	}
	// }, nil
}

func (b *Bot) updateMessageWithoutHTML(chatID int, messageID int, text string) error {
	url := fmt.Sprintf("%s/editMessageText", b.APIBaseURL)

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

func (b *Bot) SendMessageAndGetID(chatID int, text string) (*Message, error) {
	url := fmt.Sprintf("%s/sendMessage", b.APIBaseURL)

	htmlText := format.ConvertToTelegramHTML(text)

	payload := SendMessageRequest{
		ChatID:    chatID,
		Text:      htmlText,
		ParseMode: "HTML",
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

func (b *Bot) HandleUpdateMessage(chatID int, messageId int, text string) error {
	err := b.UpdateMessage(chatID, messageId, text)
	if err != nil {
		log.Printf("Error updating message: %v", err)
		var teleErr *TelegramError
		if errors.As(err, &teleErr) {
			switch teleErr.Description {
			case "Bad Request: MESSAGE_TOO_LONG":
				_ = b.UpdateMessage(chatID, messageId,
					"Sorry, the response was too long for Telegram. Please try again.")
			case "Bad Request: can't parse entities":
				plainErr := b.updateMessageWithoutHTML(chatID, messageId, text)
				if plainErr != nil {
					_ = b.UpdateMessage(chatID, messageId,
						"Sorry, I encountered an error while formatting the message. Please try again.")
				}
			default:
				_ = b.UpdateMessage(chatID, messageId,
					fmt.Sprintf("Error: %s", teleErr.Description))
			}
			return nil
		}

		_ = b.UpdateMessage(chatID, messageId,
			"An unexpected error occurred. Please try again.")
	}
	return nil
}

func (b *Bot) UpdateMessage(chatID int, messageID int, text string) error {
	url := fmt.Sprintf("%s/editMessageText", b.APIBaseURL)

	htmlText := format.ConvertToTelegramHTML(text)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       htmlText,
		"parse_mode": "HTML",
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
		bodyBytes, _ := io.ReadAll(resp.Body)
		var teleErr TelegramError
		if err := json.Unmarshal(bodyBytes, &teleErr); err != nil {
			return fmt.Errorf("status %d: %s", resp.StatusCode, string(bodyBytes))
		}
		return &teleErr
	}

	return nil
}

func (e *TelegramError) Error() string {
	return fmt.Sprintf("Telegram API Error %d: %s", e.ErrorCode, e.Description)
}

func (b *Bot) SendDocument(chatID int, filePath string) error {
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

	url := fmt.Sprintf("%s/sendDocument", b.APIBaseURL)
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

func (b *Bot) SendFileWithProgress(chatID int, filePath string) error {
	if filePath == "" {
		return fmt.Errorf("invalid file path")
	}

	msg, err := b.SendMessageAndGetID(chatID, "Preparing your file...")
	if err != nil {
		return fmt.Errorf("error sending initial message: %v", err)
	}

	err = b.UpdateMessage(chatID, msg.MessageID, "Uploading file...")
	if err != nil {
		log.Printf("Error updating progress message: %v", err)
	}

	err = b.SendDocument(chatID, filePath)
	if err != nil {
		b.UpdateMessage(chatID, msg.MessageID, "Error sending file!")
		return fmt.Errorf("error sending document: %v", err)
	}

	err = b.deleteMessage(int64(chatID), msg.MessageID)
	if err != nil {
		log.Printf("Error deleting progress message: %v", err)
	}

	return nil
}

func (b *Bot) deleteMessage(chatID int64, messageID int) error {
	url := fmt.Sprintf("%s/deleteMessage", b.APIBaseURL)

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
