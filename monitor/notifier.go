package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
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
	var prevMetrics Metrics
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping monitor...")
			return
		case <-tick.C:
			current, err := FetchMetrics(config.MonitorFrequency)
			if err != nil {
				notifyError(err.Error(), listeners)
				continue
			}

			if current.IsMajorSyncing {
				notifyWarn("Node is in Major Sync", listeners)
				continue
			}

			if prevMetrics.BlockHeight.Finalized != nil &&
				current.BlockHeight.Finalized.Cmp(&prevMetrics.BlockHeight.Finalized.Int) <= 0 {
				notifyError(
					fmt.Sprintf("Node hasn't finalised new block since `%s`", prevMetrics.BlockHeight.Finalized.String()),
					listeners)
				continue
			}

			if current.Peers < 1 {
				notifyError(
					"Node has 0 peers",
					listeners)
				continue
			}

			if !current.ValidatorStats.IsValidating {
				if prevMetrics.ValidatorStats.IsValidating {
					notifyWarn(
						fmt.Sprintf("Node didn't produce blocks in last %f minutes", config.MonitorFrequency.Minutes()),
						listeners,
					)
				}
				continue
			}

			if prevMetrics.ValidatorStats.IsValidating {
				if current.ValidatorStats.LastProduced.Cmp(&prevMetrics.ValidatorStats.LastProduced.Int) <= 0 {
					notifyWarn(
						fmt.Sprintf("Node didn't produce blocks in last %f minutes", config.MonitorFrequency.Minutes()),
						listeners,
					)
				}
				continue
			}

			// all good here
			notifyOk(listeners)
			prevMetrics = current
			log.Println(prevMetrics)
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

type ValidatorStats struct {
	IsValidating bool  `json:"is_validating"`
	LastProduced *bint `json:"last_produced"`
}

func fetchValidatorStats(frequency time.Duration) (ValidatorStats, error) {
	cmd := exec.Command(
		"journalctl",
		"-u", "centrifuge",
		"-o", "json",
		"--no-pager",
		"--since", fmt.Sprintf("%d minutes ago", int(frequency.Minutes())+1))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return ValidatorStats{}, fmt.Errorf("%v: %v", err, stderr.String())
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var vs ValidatorStats
	var c int
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		nvs, err := parseValidatorLog(l)
		if err != nil {
			log.Println(err)
			continue
		}

		if nvs.LastProduced == nil {
			continue
		}

		c++
		vs = nvs
	}

	log.Println("Found:", c, "logs")
	return vs, nil
}

var valRegex = regexp.MustCompile(`🎁 Prepared block for proposing at ([0-9]+)`)

func parseValidatorLog(l string) (ValidatorStats, error) {
	var message struct {
		Message string `json:"MESSAGE"`
	}

	if err := json.Unmarshal([]byte(l), &message); err != nil {
		return ValidatorStats{}, err
	}

	res := valRegex.FindAllStringSubmatch(message.Message, -1)
	var latest *bint
	for _, s := range res {
		if len(s) > 1 {
			latest = mustBigInt(s[1])
		}
	}

	return ValidatorStats{
		IsValidating: latest != nil,
		LastProduced: latest,
	}, nil
}
