package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const helpGuide = `
**🤖 Synapse Help Guide**

Hi there! I'm **Synapse**, your versatile assistant. Here's what I can do for you:

✨ **Features and Capabilities**

	 **Create File**: Create new files ( For now, only **.txt** files are supported, other formats are coming soon! ).
	 || **Read File**: Read file ( **Comming Soon** ). ||
	 **Web Search**: Retrieve relevant information from the web.
	 **Content Extraction**: Extract data from websites.

**Need Help or Have Suggestions?**
Feel free to reach out anytime via [@harsh](https://t.me/harsh_693).

Type **/help** at anytime to revisit this guide!
`

const privacyPolicy = `
**🤖 Synapse Privacy Policy**

* Synapse uses your chat ID and text to respond.

* I temporarily store the last 10 messages for context,
but no data is permanently saved.
`

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

	log.Println(string(body))

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		return nil, err
	}
	return &update, nil
}

func (b *Bot) HandleCommands(chatId int, text string) error {
	switch {
	case text == "/start":
		b.SendMessage(chatId, "Welcome to Synapse AI chat bot")
	case text == "/help":
		b.SendMessage(chatId, helpGuide)
	case text == "/privacy":
		b.SendMessage(chatId, privacyPolicy)
	case strings.HasPrefix(text, "/"):
		b.SendMessage(chatId, "Not a vaild command. Type **/help** to see the list of available commands.")
	}
	return nil
}
