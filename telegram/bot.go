package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"tgbot/reg"
	"time"
)

type Bot struct {
	token      string
	client     http.Client
	subscriber int
	requesters []reg.Requester
}

type Response struct {
	Ok     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Chat struct {
	ID int `json:"id"`
}

func NewBot(token string, r []reg.Requester, sub int) *Bot {
	bot := &Bot{
		token:      token,
		client:     http.Client{Timeout: 5 * time.Second},
		subscriber: sub,
		requesters: r,
	}
	return bot
}

func (b *Bot) getUpdates(offset int) ([]Update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", b.token)
	payload := map[string]int{"offset": offset}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshalling payload: %v", err)
	}

	resp, err := b.client.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error fetching updates: %v", err)
	}
	defer resp.Body.Close()

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return response.Result, nil
}

func (b *Bot) sendMessage(chatID int, text string) error {
	if len(text) > 4095 {
		b.sendMessage(chatID, text[:4095])
		b.sendMessage(chatID, text[4096:])
		return nil
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.token)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling payload: %v", err)
	}

	resp, err := b.client.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error sending message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (b *Bot) Process() {
	var offset int
	for {
		updates, err := b.getUpdates(offset)
		if err != nil {
			fmt.Printf("Error fetching updates: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			chatID := update.Message.Chat.ID
			fmt.Println("new message from ", chatID)
			text := update.Message.Text

			switch text {
			case "/start":
			default:
				b.sendMessage(chatID, "проверьте терминал и найдите chatId и добавьте его в файл 'users.txt' для получения информации о квотах")
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func (b *Bot) notifySubscribers(message string) {
	b.sendMessage(b.subscriber, message)
}

func (b *Bot) StartQuotaChecker(intervalSeconds int) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()
	// for _, requester := range b.requesters {
	// 	requester.CheckAvailable()
	// }
	// // что бы бот не оповещал о квотах при каждом запуске
	for range ticker.C {
		for _, requester := range b.requesters {
			message := requester.CheckAvailable()
			if message != "" {
				b.notifySubscribers(message)
			}
		}
	}
}
