package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Bot struct {
	Token      string
	APIBaseURL string
}

func NewBot(token string) *Bot {
	return &Bot{
		Token:      token,
		APIBaseURL: "https://api.telegram.org/bot" + token,
	}
}

func (b *Bot) SetWebhook(webhookURL string) error {
	url := fmt.Sprintf("%s/setWebhook?url=%s", b.APIBaseURL, webhookURL)
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

func (b *Bot) ParseUpdate(r *http.Request) (*Update, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		return nil, err
	}
	return &update, nil
}
