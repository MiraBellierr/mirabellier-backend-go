package guestbook

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/auth"
)

func RegisterRoutes(r *gin.RouterGroup, db *sql.DB) {
	h := &handler{db: db}

	r.GET("/guestbook", h.list)
	r.POST("/guestbook", h.create)
	r.PATCH("/guestbook/:id/position", h.updatePosition)
	r.DELETE("/guestbook/:id", h.delete)
}

type handler struct {
	db *sql.DB
}

func (h *handler) list(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT g.id, g.author, g.message, g.website, g.mood, g.x, g.y, g.createdAt,
		       u.id, u.username, u.avatar
		FROM guestbook_entries g
		LEFT JOIN users u ON u.id = g.userId
		ORDER BY g.createdAt DESC
		LIMIT 100
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch entries"})
		return
	}
	defer rows.Close()

	var entries []GuestbookEntry
	for rows.Next() {
		var e GuestbookEntry
		var uid, uname, avatar sql.NullString
		rows.Scan(&e.ID, &e.Author, &e.Message, &e.Website, &e.Mood, &e.X, &e.Y, &e.CreatedAt,
			&uid, &uname, &avatar)
		if uid.Valid {
			e.User = &EntryUser{ID: uid.String, Username: uname.String}
			if avatar.Valid {
				e.User.Avatar = &avatar.String
			}
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []GuestbookEntry{}
	}

	c.JSON(http.StatusOK, entries)
}

func (h *handler) create(c *gin.Context) {
	user := auth.GetUser(c)

	var body struct {
		Name    *string `json:"name,omitempty"`
		Message *string `json:"message,omitempty"`
		Website *string `json:"website,omitempty"`
		Mood    *string `json:"mood,omitempty"`
		X       *int    `json:"x,omitempty"`
		Y       *int    `json:"y,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Node.js: author = user ? user.username : sanitizeName(req.body?.name)
	var author string
	if user != nil {
		author = user.Username
	} else {
		if body.Name == nil || strings.TrimSpace(*body.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
			return
		}
		author = sanitizeText(*body.Name, 40)
		if author == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
			return
		}
	}

	if body.Message == nil || strings.TrimSpace(*body.Message) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message is required"})
		return
	}
	message := sanitizeText(*body.Message, 400)

	// Node.js: website = user ? null : sanitizeWebsite(req.body?.website)
	var website *string
	if user == nil && body.Website != nil && strings.TrimSpace(*body.Website) != "" {
		w := strings.TrimSpace(*body.Website)
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			website = &w
		}
	}

	var mood string
	if body.Mood != nil {
		mood = strings.ToLower(strings.TrimSpace(*body.Mood))
	}
	if mood == "" || !validMoods[mood] {
		mood = "sparkly"
	}

	// Position — use fallback if not provided
	var x, y int
	existingCount := 0
	h.db.QueryRow("SELECT COUNT(*) FROM guestbook_entries").Scan(&existingCount)
	fallbackX, fallbackY := calculateFallbackPosition(existingCount)

	if body.X != nil {
		x = *body.X
	} else {
		x = fallbackX
	}
	if body.Y != nil {
		y = *body.Y
	} else {
		y = fallbackY
	}

	id := generateEntryID()
	var userID *string
	if user != nil {
		userID = &user.ID
	}

	_, err := h.db.Exec(`
		INSERT INTO guestbook_entries (id, userId, author, message, website, mood, x, y, createdAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, id, userID, author, message, website, mood, x, y)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create entry"})
		return
	}

	entry := GuestbookEntry{
		ID:      id,
		Author:  author,
		Message: message,
		Website: website,
		Mood:    mood,
		X:       x,
		Y:       y,
	}
	if user != nil {
		entry.User = &EntryUser{ID: user.ID, Username: user.Username, Avatar: user.Avatar}
	}

	c.JSON(http.StatusCreated, entry)
}

func sanitizeText(value string, maxLen int) string {
	s := strings.TrimSpace(value)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func (h *handler) updatePosition(c *gin.Context) {
	id := c.Param("id")
	var input UpdatePositionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Position x and y required"})
		return
	}

	_, err := h.db.Exec("UPDATE guestbook_entries SET x = ?, y = ? WHERE id = ?", input.X, input.Y, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"x": input.X, "y": input.Y})
}

func (h *handler) delete(c *gin.Context) {
	id := c.Param("id")

	user := auth.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var ownerID sql.NullString
	err := h.db.QueryRow("SELECT userId FROM guestbook_entries WHERE id = ?", id).Scan(&ownerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found"})
		return
	}

	if !ownerID.Valid || ownerID.String != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	h.db.Exec("DELETE FROM guestbook_entries WHERE id = ?", id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
