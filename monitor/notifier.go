package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Severity int

const (
	Info Severity = iota
	Warn
	Alert

	OkayEmoji  = "✅"
	WarnEmoji  = "⚠️"
	ErrorEmoji = "❌"
)

type Listener interface {
	Start(ctx context.Context)
	Notify(severity Severity, message string)
}

func InitMonitor(ctx context.Context, config Config, listeners []Listener) {
	log.Println("Starting monitoring....")
	log.Printf("Checking every %s...\n", config.MonitorFrequency)

	tick := time.NewTicker(config.MonitorFrequency)
	var previous Metrics
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping monitor...")
			return
		case <-tick.C:
			current, err := FetchMetrics()
			if err != nil {
				notifyError(err.Error(), listeners)
				continue
			}

			if current.IsMajorSyncing {
				notifyWarn("Node is in Major Sync", listeners)
				continue
			}

			if previous.BlockHeight.Finalized != nil &&
				current.BlockHeight.Finalized.Cmp(&previous.BlockHeight.Finalized.Int) <= 0 {
				notifyError(
					fmt.Sprintf("Node hasn't finalised new block since `%s`", previous.BlockHeight.Finalized.String()),
					listeners)
				continue
			}

			if current.Peers < 1 {
				notifyError(
					"Node has 0 peers",
					listeners)
				continue
			}

			// all good here
			notifyOk(listeners)
			previous = current
			log.Println(previous)
		}
	}
}

func notifyOk(listeners []Listener) {
	notify(Info, listeners, "Ok")
}

func notifyWarn(message string, listeners []Listener) {
	notify(Warn, listeners, message)
}

func notifyError(message string, listeners []Listener) {
	notify(Alert, listeners, message)
}

func notify(severity Severity, listeners []Listener, msg string) {
	for _, listener := range listeners {
		listener.Notify(severity, msg)
	}
}
