package qotd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/mirabellier/mirabellier-backend-go/internal/config"
)

func StartDiscordScheduler(db *sql.DB, cfg *config.Config) {
	if cfg.QOTDDiscordWebhookURL == "" {
		return
	}

	var mu sync.Mutex

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			mu.Lock()
			maybeNotify(db, cfg)
			mu.Unlock()
		}
	}()
}

func maybeNotify(db *sql.DB, cfg *config.Config) {
	today := time.Now().UTC().Format("2006-01-02")

	var prompt string
	var discordNotifiedAt *string
	err := db.QueryRow(`
		SELECT prompt, discordNotifiedAt
		FROM daily_questions
		WHERE recordedDate = ? AND archived_at IS NULL
	`, today).Scan(&prompt, &discordNotifiedAt)
	if err != nil || discordNotifiedAt != nil {
		return
	}

	payload := map[string]interface{}{
		"username": cfg.QOTDDiscordWebhookUsername,
		"content":  "**Question of the Day**\n\n" + prompt,
	}
	if cfg.QOTDDiscordWebhookAvatarURL != "" {
		payload["avatar_url"] = cfg.QOTDDiscordWebhookAvatarURL
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(cfg.QOTDDiscordWebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		db.Exec("UPDATE daily_questions SET discordNotifiedAt = datetime('now') WHERE recordedDate = ?", today)
	}
}
