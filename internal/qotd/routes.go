package qotd

import (
	"database/sql"
	"net/http"
	"strconv"
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
	OwnerDiscordIDs             []string
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

func (h *handler) isOwner(user *auth.User) bool {
	if user == nil || user.DiscordID == nil { return false }
	for _, id := range h.cfg.OwnerDiscordIDs {
		if *user.DiscordID == id { return true }
	}
	return false
}

func (h *handler) seoPage(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (h *handler) embedImage(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

func (h *handler) getActiveRecordedDate() string {
	today := time.Now().UTC().Format("2006-01-02")

	var active string
	// Carry-forward: find oldest unanswered (or locked-today) question on or before today
	// Node.js: SELECT ... WHERE q.recordedDate <= ? ... ORDER BY q.recordedDate ASC LIMIT 1
	err := h.db.QueryRow(`
		SELECT q.recordedDate FROM daily_questions q
		LEFT JOIN daily_question_answers a ON a.recordedDate = q.recordedDate
		WHERE q.recordedDate <= ? AND q.archivedAt IS NULL
		GROUP BY q.recordedDate
		HAVING COUNT(a.id) = 0 OR substr(COALESCE(q.lockedAt,''), 1, 10) = ?
		ORDER BY q.recordedDate ASC LIMIT 1
	`, today, today).Scan(&active)
	if err == nil && active != "" {
		return active
	}

	// Fallback: today's question if exists and not archived
	err = h.db.QueryRow(`SELECT recordedDate FROM daily_questions WHERE recordedDate = ? AND archivedAt IS NULL`, today).Scan(&active)
	if err == nil && active != "" {
		return active
	}

	return today
}

func (h *handler) getCurrent(c *gin.Context) {
	today := time.Now().UTC().Format("2006-01-02")

	activeDate := h.getActiveRecordedDate()

	var prompt string
	var lockedAt, archivedAt sql.NullString
	var questionCreatedAt, questionUpdatedAt string
	err := h.db.QueryRow(`SELECT prompt, lockedAt, archivedAt, createdAt, updatedAt FROM daily_questions WHERE recordedDate = ?`, activeDate).
		Scan(&prompt, &lockedAt, &archivedAt, &questionCreatedAt, &questionUpdatedAt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"currentRecordedDate": today,
			"question":            nil,
			"answers":             []interface{}{},
			"canAnswer":           false,
			"hasAnswered":         false,
			"viewerMode":          "guest",
		})
		return
	}

	// Fetch answers for the active question's recordedDate
	rows, err := h.db.Query(`
		SELECT a.id, a.userId, a.guestName, a.answer, a.createdAt,
		       u.username, u.avatar
		FROM daily_question_answers a
		LEFT JOIN users u ON u.id = a.userId
		WHERE a.recordedDate = ?
		ORDER BY a.createdAt ASC
	`, activeDate)
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
		a.RecordedDate = activeDate
		var userID sql.NullString
		var username, avatar sql.NullString
		var guestName sql.NullString
		var answerText, createdAt, id string
		rows.Scan(&id, &userID, &guestName, &answerText, &createdAt, &username, &avatar)
		a.ID = id
		a.Answer = answerText
		a.CreatedAt = createdAt
		if guestName.Valid { a.GuestName = &guestName.String }
		if userID.Valid && username.Valid {
			a.User = &struct {
				ID       string  `json:"id,omitempty"`
				Username string  `json:"username,omitempty"`
				Avatar   *string `json:"avatar,omitempty"`
			}{ID: userID.String, Username: username.String}
			if avatar.Valid { a.User.Avatar = &avatar.String }
		}
		answers = append(answers, a)
	}
	if answers == nil { answers = []Answer{} }

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

	// Build question object
	q := map[string]any{
		"recordedDate": activeDate,
		"prompt":       prompt,
		"createdAt":    questionCreatedAt,
		"updatedAt":    questionUpdatedAt,
	}
	if lockedAt.Valid { q["lockedAt"] = lockedAt.String } else { q["lockedAt"] = nil }
	if archivedAt.Valid { q["archivedAt"] = archivedAt.String } else { q["archivedAt"] = nil }

	c.JSON(http.StatusOK, gin.H{
		"currentRecordedDate": activeDate,
		"question":            q,
		"answers":             answers,
		"canAnswer":           !hasAnswered,
		"hasAnswered":         hasAnswered,
		"viewerMode":          viewerMode,
	})
}

func (h *handler) setCurrentPrompt(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !h.isOwner(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	var input struct {
		Prompt string `json:"prompt" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt is required"})
		return
	}

	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" || len(prompt) > 240 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt must be 1-240 characters"})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	now := time.Now().UTC().Format(time.RFC3339)

	// Node.js: upsertQuestion — INSERT or UPDATE if exists and not locked
	result, err := h.db.Exec(`
		INSERT INTO daily_questions (recordedDate, prompt, createdByUserId, lockedAt, archivedAt, createdAt, updatedAt)
		VALUES (?, ?, ?, NULL, NULL, ?, ?)
		ON CONFLICT(recordedDate) DO UPDATE SET
			prompt = excluded.prompt,
			createdByUserId = excluded.createdByUserId,
			updatedAt = excluded.updatedAt
		WHERE lockedAt IS NULL
	`, today, prompt, user.ID, now, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set prompt"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Question is locked and cannot be edited"})
		return
	}

	// Return the updated question
	var qPrompt, ca, ua string
	var la, aa sql.NullString
	h.db.QueryRow(`SELECT prompt, lockedAt, archivedAt, createdAt, updatedAt FROM daily_questions WHERE recordedDate = ?`, today).
		Scan(&qPrompt, &la, &aa, &ca, &ua)
	q := map[string]any{
		"recordedDate": today, "prompt": qPrompt, "createdAt": ca, "updatedAt": ua,
		"answerCount": 0, "isCurrent": true,
	}
	if la.Valid { q["lockedAt"] = la.String } else { q["lockedAt"] = nil }
	if aa.Valid { q["archivedAt"] = aa.String } else { q["archivedAt"] = nil }

	c.JSON(http.StatusOK, map[string]any{"question": q})
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

	activeDate := h.getActiveRecordedDate()
	user := auth.GetUser(c)

	answerID := "qa_" + time.Now().UTC().Format("20060102150405") + "_" + randomHex(4)

	if user != nil {
		_, err := h.db.Exec(`
			INSERT INTO daily_question_answers (id, recordedDate, userId, identityType, identityKey, answer, createdAt)
			VALUES (?, ?, ?, 'user', ?, ?, datetime('now'))
		`, answerID, activeDate, user.ID, user.ID, strings.TrimSpace(input.Answer))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "You have already answered today's question"})
			return
		}

		// Lock question
		h.db.Exec("UPDATE daily_questions SET lockedAt = datetime('now') WHERE recordedDate = ? AND lockedAt IS NULL", activeDate)
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
		`, answerID, activeDate, name, guestToken, strings.TrimSpace(input.Answer))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "This guest has already answered today's question"})
			return
		}

		h.db.Exec("UPDATE daily_questions SET lockedAt = datetime('now') WHERE recordedDate = ? AND lockedAt IS NULL", activeDate)
	}

	c.JSON(http.StatusCreated, gin.H{"ok": true})
}

func (h *handler) adminQueue(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !h.isOwner(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	startDate := h.getActiveRecordedDate()

	// Pagination
	pageSize := 5
	if ps := c.Query("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n >= 1 && n <= 50 {
			pageSize = n
		}
	}

	var totalQuestions int
	h.db.QueryRow(`SELECT COUNT(*) FROM daily_questions WHERE recordedDate >= ? AND archivedAt IS NULL`, startDate).Scan(&totalQuestions)

	totalPages := 0
	if totalQuestions > 0 {
		totalPages = (totalQuestions + pageSize - 1) / pageSize
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1 && n <= totalPages {
			page = n
		}
	}

	offset := (page - 1) * pageSize

	rows, err := h.db.Query(`
		SELECT q.recordedDate, q.prompt, q.lockedAt, q.archivedAt, q.createdAt, q.updatedAt,
		       COALESCE(COUNT(a.id), 0) AS answerCount
		FROM daily_questions q
		LEFT JOIN daily_question_answers a ON a.recordedDate = q.recordedDate
		WHERE q.recordedDate >= ? AND q.archivedAt IS NULL
		GROUP BY q.recordedDate, q.prompt, q.lockedAt, q.archivedAt, q.createdAt, q.updatedAt
		ORDER BY q.recordedDate ASC
		LIMIT ? OFFSET ?
	`, startDate, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch questions"})
		return
	}
	defer rows.Close()

	var questions []map[string]any
	for rows.Next() {
		var recordedDate, prompt, createdAt, updatedAt string
		var lockedAt, archivedAt sql.NullString
		var answerCount int
		rows.Scan(&recordedDate, &prompt, &lockedAt, &archivedAt, &createdAt, &updatedAt, &answerCount)
		q := map[string]any{
			"recordedDate": recordedDate,
			"prompt":       prompt,
			"createdAt":    createdAt,
			"updatedAt":    updatedAt,
			"answerCount":  answerCount,
			"isCurrent":    recordedDate == startDate,
		}
		if lockedAt.Valid { q["lockedAt"] = lockedAt.String } else { q["lockedAt"] = nil }
		if archivedAt.Valid { q["archivedAt"] = archivedAt.String } else { q["archivedAt"] = nil }
		questions = append(questions, q)
	}
	if questions == nil { questions = []map[string]any{} }

	c.JSON(http.StatusOK, gin.H{
		"currentRecordedDate": today,
		"page":                page,
		"pageSize":            pageSize,
		"totalQuestions":      totalQuestions,
		"totalPages":          totalPages,
		"hasPreviousPage":     page > 1,
		"hasNextPage":         totalPages > 0 && page < totalPages,
		"questions":           questions,
	})
}

func (h *handler) queuePrompts(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !h.isOwner(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	var input struct {
		Prompts []string `json:"prompts" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || len(input.Prompts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Add at least one non-empty question"})
		return
	}

	// Sanitize prompts
	var prompts []string
	for _, p := range input.Prompts {
		p = strings.TrimSpace(p)
		if p != "" && len(p) <= 240 {
			prompts = append(prompts, p)
		}
	}
	if len(prompts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Add at least one non-empty question"})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	startDate := h.getActiveRecordedDate()

	// Get occupied dates from startDate onward
	rows, _ := h.db.Query(`SELECT recordedDate FROM daily_questions WHERE recordedDate >= ? ORDER BY recordedDate ASC`, startDate)
	occupied := make(map[string]bool)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			rows.Scan(&d)
			occupied[d] = true
		}
	}

	// Insert prompts, skipping occupied dates
	now := time.Now().UTC().Format(time.RFC3339)
	var addedDates []string
	nextDate := startDate

	for _, prompt := range prompts {
		for occupied[nextDate] {
			nextDate = addDaysToDate(nextDate, 1)
		}
		h.db.Exec(`INSERT OR IGNORE INTO daily_questions (recordedDate, prompt, createdByUserId, lockedAt, archivedAt, createdAt, updatedAt)
			VALUES (?, ?, ?, NULL, NULL, ?, ?)`, nextDate, prompt, user.ID, now, now)
		occupied[nextDate] = true
		addedDates = append(addedDates, nextDate)
		nextDate = addDaysToDate(nextDate, 1)
	}

	// Load the added questions
	var addedQuestions []map[string]any
	for _, d := range addedDates {
		var prompt, ca, ua string
		var la, aa sql.NullString
		err := h.db.QueryRow(`SELECT prompt, lockedAt, archivedAt, createdAt, updatedAt FROM daily_questions WHERE recordedDate = ?`, d).
			Scan(&prompt, &la, &aa, &ca, &ua)
		if err == nil {
			q := map[string]any{"recordedDate": d, "prompt": prompt, "createdAt": ca, "updatedAt": ua, "answerCount": 0, "isCurrent": d == today}
			if la.Valid { q["lockedAt"] = la.String } else { q["lockedAt"] = nil }
			if aa.Valid { q["archivedAt"] = aa.String } else { q["archivedAt"] = nil }
			addedQuestions = append(addedQuestions, q)
		}
	}
	if addedQuestions == nil { addedQuestions = []map[string]any{} }

	// Load all admin questions for response
	allRows, _ := h.db.Query(`
		SELECT q.recordedDate, q.prompt, q.lockedAt, q.archivedAt, q.createdAt, q.updatedAt,
		       COALESCE(COUNT(a.id), 0) AS answerCount
		FROM daily_questions q
		LEFT JOIN daily_question_answers a ON a.recordedDate = q.recordedDate
		WHERE q.recordedDate >= ? AND q.archivedAt IS NULL
		GROUP BY q.recordedDate, q.prompt, q.lockedAt, q.archivedAt, q.createdAt, q.updatedAt
		ORDER BY q.recordedDate ASC
	`, startDate)
	var allQuestions []map[string]any
	if allRows != nil {
		defer allRows.Close()
		for allRows.Next() {
			var rd, p, ca, ua string
			var la, aa sql.NullString
			var ac int
			allRows.Scan(&rd, &p, &la, &aa, &ca, &ua, &ac)
			q := map[string]any{"recordedDate": rd, "prompt": p, "createdAt": ca, "updatedAt": ua, "answerCount": ac, "isCurrent": rd == today}
			if la.Valid { q["lockedAt"] = la.String } else { q["lockedAt"] = nil }
			if aa.Valid { q["archivedAt"] = aa.String } else { q["archivedAt"] = nil }
			allQuestions = append(allQuestions, q)
		}
	}
	if allQuestions == nil { allQuestions = []map[string]any{} }

	c.JSON(http.StatusCreated, gin.H{
		"currentRecordedDate": today,
		"addedCount":          len(addedDates),
		"addedQuestions":      addedQuestions,
		"questions":           allQuestions,
	})
}

func addDaysToDate(dateStr string, days int) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

func (h *handler) forceArchive(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !h.isOwner(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")

	// Check if there's an active question to archive
	var prompt string
	var lockedAt sql.NullString
	err := h.db.QueryRow(`SELECT prompt, lockedAt FROM daily_questions WHERE recordedDate = ? AND archivedAt IS NULL ORDER BY recordedDate ASC LIMIT 1`, today).Scan(&prompt, &lockedAt)
	if err != nil {
		// Try carry-forward
		err = h.db.QueryRow(`SELECT prompt, lockedAt FROM daily_questions WHERE recordedDate >= ? AND archivedAt IS NULL ORDER BY recordedDate ASC LIMIT 1`, today).Scan(&prompt, &lockedAt)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No active question to archive"})
			return
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	h.db.Exec("UPDATE daily_questions SET archivedAt = ?, updatedAt = ? WHERE recordedDate = ? AND archivedAt IS NULL", now, now, today)

	// Return archived question
	var recordedDate, createdAt, updatedAt string
	var la, aa sql.NullString
	err = h.db.QueryRow(`SELECT recordedDate, prompt, lockedAt, archivedAt, createdAt, updatedAt FROM daily_questions WHERE recordedDate = ?`, today).Scan(&recordedDate, &prompt, &la, &aa, &createdAt, &updatedAt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	q := map[string]any{"recordedDate": recordedDate, "prompt": prompt, "createdAt": createdAt, "updatedAt": updatedAt, "answerCount": 0, "isCurrent": false}
	if la.Valid { q["lockedAt"] = la.String } else { q["lockedAt"] = nil }
	if aa.Valid { q["archivedAt"] = aa.String } else { q["archivedAt"] = nil }

	c.JSON(http.StatusOK, map[string]any{"archivedQuestion": q})
}

func (h *handler) archive(c *gin.Context) {
	// Archive cutoff: active question's date or today
	cutoffDate := h.getActiveRecordedDate()

	rows, err := h.db.Query(`
		SELECT q.recordedDate, q.prompt, q.createdAt, q.updatedAt,
		       COALESCE(COUNT(a.id), 0) AS answerCount
		FROM daily_questions q
		LEFT JOIN daily_question_answers a ON a.recordedDate = q.recordedDate
		WHERE q.archivedAt IS NOT NULL OR q.recordedDate < ?
		GROUP BY q.recordedDate, q.prompt, q.createdAt, q.updatedAt
		ORDER BY q.recordedDate DESC
	`, cutoffDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load question archive"})
		return
	}
	defer rows.Close()

	var entries []map[string]any
	for rows.Next() {
		var rd, prompt, ca, ua string
		var ac int
		rows.Scan(&rd, &prompt, &ca, &ua, &ac)
		entries = append(entries, map[string]any{
			"recordedDate": rd,
			"prompt":       prompt,
			"answerCount":  ac,
			"createdAt":    ca,
			"updatedAt":    ua,
		})
	}
	if entries == nil { entries = []map[string]any{} }

	c.JSON(http.StatusOK, entries)
}

func (h *handler) archiveDay(c *gin.Context) {
	recordedDate := c.Param("recordedDate")

	cutoffDate := h.getActiveRecordedDate()

	var prompt, ca, ua string
	var lockedAt, archivedAt sql.NullString
	err := h.db.QueryRow(`SELECT prompt, lockedAt, archivedAt, createdAt, updatedAt FROM daily_questions WHERE recordedDate = ? AND (archivedAt IS NOT NULL OR recordedDate < ?)`, recordedDate, cutoffDate).
		Scan(&prompt, &lockedAt, &archivedAt, &ca, &ua)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Archived question not found"})
		return
	}

	rows, err := h.db.Query(`
		SELECT a.id, a.userId, a.guestName, a.identityType, a.identityKey, a.answer, a.createdAt,
		       u.username, u.avatar
		FROM daily_question_answers a
		LEFT JOIN users u ON u.id = a.userId
		WHERE a.recordedDate = ? ORDER BY a.createdAt ASC
	`, recordedDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch answers"})
		return
	}
	defer rows.Close()

	var answers []map[string]any
	for rows.Next() {
		var id, ans, createdAt, identityType, identityKey string
		var userID, guestName, username sql.NullString
		var avatar sql.NullString
		rows.Scan(&id, &userID, &guestName, &identityType, &identityKey, &ans, &createdAt, &username, &avatar)
		a := map[string]any{"id": id, "recordedDate": recordedDate, "answer": ans, "createdAt": createdAt}
		if guestName.Valid { a["guestName"] = guestName.String }
		if username.Valid {
			uo := map[string]any{"username": username.String}
			if userID.Valid { uo["id"] = userID.String }
			if avatar.Valid { uo["avatar"] = avatar.String }
			a["user"] = uo
		}
		answers = append(answers, a)
	}
	if answers == nil { answers = []map[string]any{} }

	q := map[string]any{"recordedDate": recordedDate, "prompt": prompt, "createdAt": ca, "updatedAt": ua}
	if lockedAt.Valid { q["lockedAt"] = lockedAt.String } else { q["lockedAt"] = nil }
	if archivedAt.Valid { q["archivedAt"] = archivedAt.String } else { q["archivedAt"] = nil }

	c.JSON(http.StatusOK, map[string]any{
		"recordedDate": recordedDate,
		"question":     q,
		"answers":      answers,
		"answerCount":  len(answers),
	})
}

func (h *handler) deleteAnswer(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !h.isOwner(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM daily_question_answers WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete answer"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Answer not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func randomHex(n int) string {
	const letters = "abcdef0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UTC().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
