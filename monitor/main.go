package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/octago/sflags/gen/gflag"
)

type Config struct {
	Name             string        `json:"name"`
	MonitorFrequency time.Duration `json:"monitor_frequency"`

	TelegramKey      string `json:"telegram_key"`
	TelegramChatID   string `json:"telegram_chat_id"`
	TelegramSeverity int    `json:"telegram_severity"`

	PagerdutyAPIKey string `json:"pagerduty_api_key"`
}

func (c Config) IsTelegramBotEnabled() bool {
	return c.TelegramKey != "" && c.TelegramChatID != ""
}

func main() {
	config := Config{
		MonitorFrequency: time.Minute * 5,
		Name:             "Monitor",
	}
	err := gflag.ParseToDef(&config)
	if err != nil {
		panic(err)
	}
	flag.Parse()

	var listeners []Listener
	if config.IsTelegramBotEnabled() {
		listeners = append(listeners, NewTelegramBot(config))
	} else {
		log.Println("Telegram bot disabled.")
	}

	if config.PagerdutyAPIKey != "" {
		listeners = append(listeners, NewPagerduty(config.Name, config.PagerdutyAPIKey))
	} else {
		log.Println("Pagerduty bot disabled.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, listener := range listeners {
		go listener.Start(ctx)
	}

	go InitMonitor(ctx, config, listeners)
	select {}
}
