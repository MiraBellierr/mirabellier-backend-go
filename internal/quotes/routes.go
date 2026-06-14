package quotes

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/config"
)


func RegisterRoutes(r *gin.RouterGroup, db *sql.DB, cfg *config.Config) {
	h := &handler{db: db, cfg: cfg}

	r.GET("/quotes", h.seoPage)
	r.GET("/quote-of-the-day", h.getQuoteOfTheDay)
	r.GET("/quotes/embed-image.png", h.embedImage)
}

type handler struct {
	db  *sql.DB
	cfg *config.Config
}

func (h *handler) seoPage(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, h.cfg.FrontendURL+"/quotes")
}

func (h *handler) embedImage(c *gin.Context) {
	c.JSON(501, gin.H{"error": "not implemented"})
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (h *handler) getQuoteOfTheDay(c *gin.Context) {
	date := c.Query("date")

	if date != "" && !datePattern.MatchString(date) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format",
			"details": "Use YYYY-MM-DD for the date query parameter",
		})
		return
	}

	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	// Try exact date match first, then fallback to most recent
	var quotesJSON string
	var fetchedAt, provider, sourceType string
	var displayDate, publishedAt, fallbackReason sql.NullString

	err := h.db.QueryRow(`
		SELECT quotesJson, fetchedAt, provider, sourceType,
		       displayDate, publishedAt, fallbackReason
		FROM quote_snapshots
		WHERE recordedDate = ?
	`, date).Scan(&quotesJSON, &fetchedAt, &provider, &sourceType,
		&displayDate, &publishedAt, &fallbackReason)

	if err != nil {
		// Try fallback to most recent snapshot
		err = h.db.QueryRow(`
			SELECT quotesJson, fetchedAt, provider, sourceType,
			       displayDate, publishedAt, fallbackReason
			FROM quote_snapshots
			WHERE recordedDate <= ?
			ORDER BY recordedDate DESC LIMIT 1
		`, date).Scan(&quotesJSON, &fetchedAt, &provider, &sourceType,
			&displayDate, &publishedAt, &fallbackReason)
		if err != nil {
			c.Header("Cache-Control", "no-store, no-cache")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Quotes not found for the requested date",
			})
			return
		}
	}

	// Parse quotes from stored JSON string into array
	var quotes []map[string]any
	if err := json.Unmarshal([]byte(quotesJSON), &quotes); err != nil {
		quotes = []map[string]any{}
	}

	resp := gin.H{
		"recordedDate": date,
		"provider":     provider,
		"sourceType":   sourceType,
		"fetchedAt":    fetchedAt,
		"quotes":       quotes,
	}
	if displayDate.Valid {
		resp["displayDate"] = displayDate.String
	}
	if publishedAt.Valid {
		resp["publishedAt"] = publishedAt.String
	}
	if fallbackReason.Valid {
		resp["fallbackReason"] = fallbackReason.String
	}

	c.Header("Cache-Control", "no-store, no-cache")
	c.JSON(http.StatusOK, resp)
}

func StartQuoteScheduler(db *sql.DB, cfg *config.Config) {
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day(),
				cfg.QuoteFetchHourUTC, cfg.QuoteFetchMinuteUTC, 0, 0, time.UTC)
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			select {
			case <-time.After(time.Until(next)):
			}
			RefreshQuoteSnapshot(db, now)
		}
	}()
}
