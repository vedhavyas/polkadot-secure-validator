package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/vedhavyas/tgo"
)

type Telegram struct {
	client      *tgo.Client
	chatID      string
	botUsername string
	severity    Severity
	prevVS      ValidatorStats
	mu          sync.RWMutex
}

func NewTelegramBot(config Config) *Telegram {
	return &Telegram{
		client:      tgo.NewClient(config.TelegramKey),
		chatID:      config.TelegramChatID,
		severity:    Severity(config.TelegramSeverity),
		botUsername: config.TelegramBotUsername,
	}
}

func (t *Telegram) Start(ctx context.Context) {
	updatesChan := t.client.GetUpdatesChan(tgo.GetUpdatesParams{
		Timeout: 60,
	})

	ok, err := t.client.SetBotCommands(tgo.SetBotCommandParams{
		Commands: []tgo.BotCommand{
			{
				Command:     "metrics",
				Description: "Fetch Node metrics",
			},

			{
				Command:     "info",
				Description: "Subscribe to Info level log updates.",
			},

			{
				Command:     "warn",
				Description: "Subscribe to Warning level log updates.",
			},

			{
				Command:     "error",
				Description: "Subscribe to Error level log updates.",
			},
		},
	})
	if err != nil || !*ok {
		log.Printf("Failed to set bot commands")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updatesChan:
			if update.Message == nil {
				continue
			}

			msg := t.fetchCommand(strings.ToLower(update.Message.Text))
			switch msg {
			case "/metrics":
				log.Println("Received metrics request from the user. Sending the metrics...")
				t.sendMetrics(update.Message.ID)
			case "/info":
				log.Println("Received info log enable request from the user. Enabling...")
				t.updateSeverity(Info)
				t.sendString(update.Message.ID, fmt.Sprintf("Log level: Info %s", OkayEmoji), true)
			case "/warn":
				log.Println("Received warn enable request from the user. Disabling...")
				t.updateSeverity(Warn)
				t.sendString(update.Message.ID, fmt.Sprintf("Log level: Warn %s", WarnEmoji), true)
			case "/error":
				log.Println("Received error enable request from the user. Disabling...")
				t.updateSeverity(Alert)
				t.sendString(update.Message.ID, fmt.Sprintf("Log level: Error %s", ErrorEmoji), true)
			}
		}
	}
}

// fetchCommand intended for this bot else returns message as is
func (t *Telegram) fetchCommand(msg string) string {
	split := strings.Split(msg, "@")
	if len(split) > 1 && strings.TrimSpace(split[1]) == t.botUsername {
		return split[0]
	}

	return msg
}

func (t *Telegram) updateSeverity(severity Severity) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.severity = severity
}

func (t *Telegram) SendMessage(message string) {
	t.sendString(0, message, true)
}

func (t *Telegram) Notify(severity Severity, message string) {
	t.mu.RLock()
	current := t.severity
	t.mu.RUnlock()

	if current > severity {
		return
	}

	switch severity {
	case Info:
		t.sendString(0, wrapMessage(OkayEmoji, message), false)
	case Warn:
		t.sendString(0, wrapMessage(WarnEmoji, message), true)
	default:
		t.sendString(0, wrapMessage(ErrorEmoji, message), true)
	}
}

func wrapMessage(emoji, message string) string {
	return fmt.Sprintf("Status: %s\n%s", emoji, message)
}
func (t *Telegram) sendMetrics(replyID int) {
	metrics, err := FetchMetrics(t.prevVS.cursor)
	if err != nil {
		t.sendString(replyID, wrapMessage(ErrorEmoji, err.Error()), true)
		return
	}

	if metrics.ValidatorStats.LastProduced == nil {
		metrics.ValidatorStats.LastProduced = t.prevVS.LastProduced
		metrics.ValidatorStats.IsValidating = t.prevVS.IsValidating
	}

	str := metrics.String()
	_, err = t.client.SendMessage(tgo.SendMessageParams{
		ChatID:                t.chatID,
		Text:                  str,
		ParseMode:             "MarkdownV2",
		DisableWebPagePreview: true,
		ReplyToMessageID:      replyID,
	})
	if err != nil {
		log.Printf("failed to send metrics to telegram bot: %v\n", err)
	}
	t.prevVS = metrics.ValidatorStats
}

func (t *Telegram) sendString(replyID int, msg string, notify bool) {
	_, err := t.client.SendMessage(tgo.SendMessageParams{
		ChatID:                t.chatID,
		Text:                  msg,
		DisableWebPagePreview: true,
		ReplyToMessageID:      replyID,
		DisableNotification:   !notify,
	})
	if err != nil {
		log.Printf("failed to send message to telegram bot: %v\n", err)
	}
}
