package qotd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const checkIntervalMs = 60 * 1000
const discordEmbedColor = 16738740

type DiscordConfig struct {
	QOTDDiscordWebhookURL       string
	QOTDDiscordWebhookUsername  string
	QOTDDiscordWebhookAvatarURL string
	WebsiteBase                 string
}

func StartDiscordScheduler(db *sql.DB, cfg *DiscordConfig) {
	if cfg.QOTDDiscordWebhookURL == "" {
		return
	}

	// Startup check
	go func() {
		result, err := maybeNotifyNewQuestion(db, cfg)
		if err != nil {
			log.Printf("[qotd-discord] %v", err)
		} else if result.skipped {
			log.Printf("[qotd-discord] skipped: %s", result.reason)
		} else {
			log.Printf("[qotd-discord] Sent Discord notification for %s", result.recordedDate)
		}
	}()

	// Poll every 60 seconds
	go func() {
		ticker := time.NewTicker(checkIntervalMs * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			result, err := maybeNotifyNewQuestion(db, cfg)
			if err != nil {
				log.Printf("[qotd-discord] %v", err)
			} else if !result.skipped {
				log.Printf("[qotd-discord] Sent Discord notification for %s", result.recordedDate)
			}
		}
	}()
}

type notifyResult struct {
	skipped      bool
	reason       string
	recordedDate string
}

var inFlightNotify sync.Mutex

func maybeNotifyNewQuestion(db *sql.DB, cfg *DiscordConfig) (notifyResult, error) {
	if !inFlightNotify.TryLock() {
		return notifyResult{skipped: true, reason: "notification already in flight"}, nil
	}
	defer inFlightNotify.Unlock()

	today := time.Now().UTC().Format("2006-01-02")

	// Find the active question (carry-forward)
	active := getActiveQuestionForDiscord(db, today)
	if active == nil {
		return notifyResult{skipped: true, reason: "no active question"}, nil
	}

	if active.discordNotifiedAt != "" {
		return notifyResult{skipped: true, reason: "already notified", recordedDate: active.recordedDate}, nil
	}

	// Build and send webhook
	payload := buildWebhookPayload(active, today, cfg)
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(cfg.QOTDDiscordWebhookURL, "application/json; charset=utf-8", bytes.NewReader(body))
	if err != nil {
		return notifyResult{}, fmt.Errorf("webhook request failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return notifyResult{}, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	// Mark as notified
	notifiedAt := time.Now().UTC().Format(time.RFC3339)
	db.Exec("UPDATE daily_questions SET discordNotifiedAt = COALESCE(discordNotifiedAt, ?) WHERE recordedDate = ?",
		notifiedAt, active.recordedDate)

	return notifyResult{recordedDate: active.recordedDate}, nil
}

type activeQuestion struct {
	recordedDate       string
	prompt             string
	discordNotifiedAt  string
}

func getActiveQuestionForDiscord(db *sql.DB, today string) *activeQuestion {
	// Reuse shared carry-forward date helper
	activeDate := getActiveRecordedDate(db)

	var q activeQuestion
	err := db.QueryRow(`
		SELECT recordedDate, prompt, COALESCE(discordNotifiedAt, '')
		FROM daily_questions WHERE recordedDate = ? AND archivedAt IS NULL
	`, activeDate).Scan(&q.recordedDate, &q.prompt, &q.discordNotifiedAt)
	if err != nil {
		return nil
	}

	return &q
}

func buildWebhookPayload(q *activeQuestion, today string, cfg *DiscordConfig) map[string]any {
	questionURL := fmt.Sprintf("%s/question-of-the-day", cfg.WebsiteBase)
	footerText := "Fresh drop"
	if q.recordedDate != today {
		footerText = fmt.Sprintf("Carried forward from %s and now live", q.recordedDate)
	}

	payload := map[string]any{
		"content": "A new Question of the Day just dropped.",
		"allowed_mentions": map[string]any{
			"parse": []string{},
		},
		"embeds": []map[string]any{
			{
				"title":       "New Question of the Day",
				"url":         questionURL,
				"description": q.prompt,
				"color":       discordEmbedColor,
				"fields": []map[string]any{
					{"name": "Recorded date", "value": q.recordedDate, "inline": true},
					{"name": "Open", "value": fmt.Sprintf("[Answer on Mirabellier.com](%s)", questionURL), "inline": true},
				},
				"footer":    map[string]any{"text": footerText},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	if cfg.QOTDDiscordWebhookUsername != "" {
		payload["username"] = cfg.QOTDDiscordWebhookUsername
	}
	if cfg.QOTDDiscordWebhookAvatarURL != "" {
		payload["avatar_url"] = cfg.QOTDDiscordWebhookAvatarURL
	}

	return payload
}
