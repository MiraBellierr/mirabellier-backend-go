package qotd

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/auth"
)

// Config is the subset needed by QOTD package.
type Config struct {
	QOTDDiscordWebhookURL       string
	QOTDDiscordWebhookUsername  string
	QOTDDiscordWebhookAvatarURL string
}

func RegisterRoutes(r *gin.RouterGroup, db *sql.DB, cfg *Config) {
	h := &handler{db: db, cfg: cfg}

	r.GET("/question-of-the-day", h.seoPage)
	r.GET("/question-of-the-day/embed-image.png", h.embedImage)
	r.GET("/question-of-the-day/current", h.getCurrent)
	r.POST("/question-of-the-day/current", auth.Require(), h.setCurrentPrompt)
	r.POST("/question-of-the-day/current/answers", h.submitAnswer)
	r.GET("/question-of-the-day/admin/questions", auth.Require(), h.adminQueue)
	r.POST("/question-of-the-day/admin/questions", auth.Require(), h.queuePrompts)
	r.POST("/question-of-the-day/admin/current/force-archive", auth.Require(), h.forceArchive)
	r.GET("/question-of-the-day/archive", h.archive)
	r.GET("/question-of-the-day/archive/:recordedDate", h.archiveDay)
	r.DELETE("/question-of-the-day/answers/:id", auth.Require(), h.deleteAnswer)
}

type handler struct {
	db  *sql.DB
	cfg *Config
}

func (h *handler) seoPage(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (h *handler) embedImage(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

func (h *handler) getCurrent(c *gin.Context) {
	today := time.Now().UTC().Format("2006-01-02")

	// Carry-forward logic: if today has no question, check for unanswered future ones
	var prompt string
	var lockedAt sql.NullString
	err := h.db.QueryRow(`
		SELECT prompt, lockedAt FROM daily_questions
		WHERE recordedDate = ? AND archivedAt IS NULL
	`, today).Scan(&prompt, &lockedAt)
	if err != nil {
		// Try carry-forward from future
		err = h.db.QueryRow(`
			SELECT prompt, lockedAt FROM daily_questions
			WHERE recordedDate > ? AND archivedAt IS NULL AND lockedAt IS NULL
			ORDER BY recordedDate ASC LIMIT 1
		`, today).Scan(&prompt, &lockedAt)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"currentRecordedDate": nil, "question": nil, "answers": []interface{}{}})
			return
		}
	}

	// Fetch answers
	rows, err := h.db.Query(`
		SELECT a.id, a.userId, a.guestName, a.identityType, a.identityKey, a.answer, a.createdAt,
		       u.username, u.avatar
		FROM daily_question_answers a
		LEFT JOIN users u ON u.id = a.userId
		WHERE a.recordedDate = ?
		ORDER BY a.createdAt ASC
	`, today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch answers"})
		return
	}
	defer rows.Close()

	type Answer struct {
		ID           string  `json:"id"`
		RecordedDate string  `json:"recordedDate"`
		GuestName    *string `json:"guestName,omitempty"`
		Answer       string  `json:"answer"`
		CreatedAt    string  `json:"createdAt"`
		User         *struct {
			ID       string  `json:"id,omitempty"`
			Username string  `json:"username,omitempty"`
			Avatar   *string `json:"avatar,omitempty"`
		} `json:"user,omitempty"`
	}

	var answers []Answer
	for rows.Next() {
		var a Answer
		a.RecordedDate = today
		var userID sql.NullString
		var username, avatar sql.NullString
		var guestName sql.NullString
		var answerText, createdAt, id string
		var identityType, identityKey string
		rows.Scan(&id, &userID, &guestName, &identityType, &identityKey, &answerText, &createdAt, &username, &avatar)
		a.ID = id
		a.Answer = answerText
		a.CreatedAt = createdAt
		if guestName.Valid {
			a.GuestName = &guestName.String
		}
		if userID.Valid && username.Valid {
			a.User = &struct {
				ID       string  `json:"id,omitempty"`
				Username string  `json:"username,omitempty"`
				Avatar   *string `json:"avatar,omitempty"`
			}{ID: userID.String, Username: username.String}
			if avatar.Valid {
				a.User.Avatar = &avatar.String
			}
		}
		answers = append(answers, a)
	}
	if answers == nil {
		answers = []Answer{}
	}

	// Viewer state
	hasAnswered := false
	viewerMode := "guest"
	user := auth.GetUser(c)
	if user != nil {
		viewerMode = "user"
		for _, a := range answers {
			if a.User != nil && a.User.ID == user.ID {
				hasAnswered = true
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"currentRecordedDate": today,
		"question": gin.H{
			"recordedDate": today,
			"prompt":       prompt,
			"lockedAt":     lockedAt,
			"archivedAt":   nil,
			"createdAt":    today,
			"updatedAt":    today,
		},
		"answers":     answers,
		"canAnswer":   !hasAnswered,
		"hasAnswered": hasAnswered,
		"viewerMode":  viewerMode,
	})
}

func (h *handler) setCurrentPrompt(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !auth.IsOwner(user, nil) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	var input struct {
		Prompt string `json:"prompt" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt is required"})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	if len(strings.TrimSpace(input.Prompt)) > 240 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt must be 240 characters or less"})
		return
	}

	_, err := h.db.Exec(`
		INSERT INTO daily_questions (recordedDate, prompt, createdByUserId, createdAt, updatedAt)
		VALUES (?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(recordedDate) DO UPDATE SET
			prompt = excluded.prompt,
			updatedAt = datetime('now')
		WHERE lockedAt IS NULL
	`, today, strings.TrimSpace(input.Prompt), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set prompt"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *handler) submitAnswer(c *gin.Context) {
	var input struct {
		Answer     string `json:"answer" binding:"required"`
		Name       string `json:"name,omitempty"`
		GuestToken string `json:"guestToken,omitempty"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Answer is required"})
		return
	}

	if len(strings.TrimSpace(input.Answer)) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Answer must be 500 characters or less"})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	user := auth.GetUser(c)

	answerID := "qa_" + time.Now().UTC().Format("20060102150405") + "_" + randomHex(4)

	if user != nil {
		_, err := h.db.Exec(`
			INSERT INTO daily_question_answers (id, recordedDate, userId, identityType, identityKey, answer, createdAt)
			VALUES (?, ?, ?, 'user', ?, ?, datetime('now'))
		`, answerID, today, user.ID, user.ID, strings.TrimSpace(input.Answer))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "You have already answered today's question"})
			return
		}

		// Lock question
		h.db.Exec("UPDATE daily_questions SET lockedAt = datetime('now') WHERE recordedDate = ? AND lockedAt IS NULL", today)
	} else {
		guestToken := strings.TrimSpace(input.GuestToken)
		if guestToken == "" || !strings.HasPrefix(guestToken, "qotd:guest:") || len(guestToken) < 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Valid guest token required"})
			return
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required for guests"})
			return
		}

		_, err := h.db.Exec(`
			INSERT INTO daily_question_answers (id, recordedDate, guestName, identityType, identityKey, answer, createdAt)
			VALUES (?, ?, ?, 'guest', ?, ?, datetime('now'))
		`, answerID, today, name, guestToken, strings.TrimSpace(input.Answer))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "This guest has already answered today's question"})
			return
		}

		h.db.Exec("UPDATE daily_questions SET lockedAt = datetime('now') WHERE recordedDate = ? AND lockedAt IS NULL", today)
	}

	c.JSON(http.StatusCreated, gin.H{"ok": true})
}

func (h *handler) adminQueue(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"questions": []interface{}{}, "total": 0})
}

func (h *handler) queuePrompts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *handler) forceArchive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *handler) archive(c *gin.Context) {
	c.JSON(http.StatusOK, []interface{}{})
}

func (h *handler) archiveDay(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func (h *handler) deleteAnswer(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func randomHex(n int) string {
	const letters = "abcdef0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
