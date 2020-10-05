package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Pagerduty struct {
	Name   string
	APIKey string
}

func NewPagerduty(name, apiKey string) *Pagerduty {
	return &Pagerduty{
		Name:   name,
		APIKey: apiKey,
	}
}

type payload struct {
	Summary   string `json:"summary"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Severity  string `json:"severity"`
}

type pgEventRequestParams struct {
	RoutingKey  string  `json:"routing_key"`
	EventAction string  `json:"event_action"`
	Payload     payload `json:"payload"`
}

func (p *Pagerduty) Start(ctx context.Context) {
	return
}

func (p *Pagerduty) Notify(severity Severity, message string) {
	if severity != Alert {
		return
	}

	params := pgEventRequestParams{
		RoutingKey:  p.APIKey,
		EventAction: "trigger",
		Payload: payload{
			Summary:   message,
			Timestamp: time.Now().Format(time.RFC3339),
			Source:    p.Name,
			Severity:  "critical",
		},
	}

	d, err := json.Marshal(params)
	if err != nil {
		panic(err)
	}

	url := "https://events.pagerduty.com/v2/enqueue"
	resp, err := http.Post(url, "application/json", bytes.NewReader(d))
	if err != nil {
		log.Printf("failed to send alert to pagerduty: %v\n", err)
		return
	}

	defer resp.Body.Close()
	d, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read pagerduty response: %v\n", err)
		return
	}

	log.Printf("Pagerduty response: %s\n", string(d))
}
