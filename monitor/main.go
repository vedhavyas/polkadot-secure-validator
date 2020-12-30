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

	TelegramKey         string `json:"telegram_key"`
	TelegramChatID      string `json:"telegram_chat_id"`
	TelegramSeverity    int    `json:"telegram_severity"`
	TelegramBotUsername string `json:"telegram_bot_username"`

	PagerdutyAPIKey string `json:"pagerduty_api_key"`

	Payout struct {
		Stash        string `json:"stash"`
		HotWalletURI string `json:"hot_wallet_uri"`
		Decimals     int    `json:"decimals"`
		Unit         string `json:"unit"`
	} `json:"payout"`
}

func (c Config) IsTelegramBotEnabled() bool {
	return c.TelegramKey != "" && c.TelegramChatID != ""
}

func main() {
	config := Config{
		MonitorFrequency: time.Minute * 5,
		Name:             "Monitor",
	}
	config.Payout.Decimals = 1
	err := gflag.ParseToDef(&config)
	if err != nil {
		panic(err)
	}
	flag.Parse()

	var listeners []Listener
	var telegram *Telegram
	if config.IsTelegramBotEnabled() {
		telegram = NewTelegramBot(config)
		listeners = append(listeners, telegram)
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

	if config.Payout.Stash != "" || config.Payout.HotWalletURI != "" {
		log.Println("Starting Accountant...")
		acc, err := NewAccountant(config.Payout.Stash,
			config.Payout.HotWalletURI,
			config.Payout.Unit,
			config.Payout.Decimals, listeners)
		if err != nil {
			log.Println("Failed to create accountant", err)
		}

		if telegram != nil {
			telegram.SetAccountant(acc)
		}

		err = acc.Start(ctx)
		if err != nil {
			log.Println("Failed to start accountant", err)
		}
	}

	select {}
}
