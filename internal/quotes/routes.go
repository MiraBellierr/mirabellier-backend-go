package quotes

import (
	"database/sql"
	"encoding/json"
	"log"
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
	requestedDate := c.Query("date")

	if requestedDate != "" && !datePattern.MatchString(requestedDate) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format",
			"details": "Use YYYY-MM-DD for the date query parameter",
		})
		return
	}

	today := time.Now().UTC().Format("2006-01-02")

	// If requesting a past date, return from DB only (no fetch)
	if requestedDate != "" && requestedDate != today {
		snapshot := getQuoteSnapshot(h.db, requestedDate)
		if snapshot == nil {
			c.Header("Cache-Control", "no-store, no-cache")
			c.JSON(http.StatusNotFound, gin.H{"error": "Quotes not found for the requested date"})
			return
		}
		c.Header("Cache-Control", "no-store, no-cache")
		c.JSON(http.StatusOK, snapshot)
		return
	}

	// For today: try to fetch fresh quotes, fallback to DB
	snapshot, err := ensureTodaysQuote(h.db)
	if err != nil {
		snapshot = getLatestQuoteSnapshot(h.db)
		if snapshot == nil {
			c.Header("Cache-Control", "no-store, no-cache")
			c.JSON(http.StatusNotFound, gin.H{"error": "Quotes not found for the requested date"})
			return
		}
		snapshot["stale"] = true
		snapshot["staleReason"] = err.Error()
	} else if requestedDate == "" && snapshot == nil {
		// No date specified, no snapshot for today — try latest
		snapshot = getLatestQuoteSnapshot(h.db)
		if snapshot == nil {
			c.Header("Cache-Control", "no-store, no-cache")
			c.JSON(http.StatusNotFound, gin.H{"error": "Quotes not found"})
			return
		}
	}

	c.Header("Cache-Control", "no-store, no-cache")
	c.JSON(http.StatusOK, snapshot)
}

func getQuoteSnapshot(db *sql.DB, recordedDate string) map[string]any {
	var quotesJSON, fetchedAt, provider, sourceType string
	var displayDate, publishedAt, fallbackReason sql.NullString
	err := db.QueryRow(`
		SELECT quotesJson, fetchedAt, provider, sourceType,
		       displayDate, publishedAt, fallbackReason
		FROM quote_snapshots WHERE recordedDate = ?
	`, recordedDate).Scan(&quotesJSON, &fetchedAt, &provider, &sourceType,
		&displayDate, &publishedAt, &fallbackReason)
	if err != nil {
		return nil
	}
	var quotes []map[string]any
	json.Unmarshal([]byte(quotesJSON), &quotes)
	resp := map[string]any{
		"recordedDate": recordedDate,
		"provider":     provider,
		"sourceType":   sourceType,
		"fetchedAt":    fetchedAt,
		"quotes":       quotes,
	}
	if displayDate.Valid { resp["displayDate"] = displayDate.String }
	if publishedAt.Valid { resp["publishedAt"] = publishedAt.String }
	if fallbackReason.Valid { resp["fallbackReason"] = fallbackReason.String }
	return resp
}

func getLatestQuoteSnapshot(db *sql.DB) map[string]any {
	today := time.Now().UTC().Format("2006-01-02")
	var recordedDate string
	var quotesJSON, fetchedAt, provider, sourceType string
	var displayDate, publishedAt, fallbackReason sql.NullString
	err := db.QueryRow(`
		SELECT recordedDate, quotesJson, fetchedAt, provider, sourceType,
		       displayDate, publishedAt, fallbackReason
		FROM quote_snapshots
		WHERE recordedDate <= ? ORDER BY recordedDate DESC LIMIT 1
	`, today).Scan(&recordedDate, &quotesJSON, &fetchedAt, &provider, &sourceType,
		&displayDate, &publishedAt, &fallbackReason)
	if err != nil {
		return nil
	}
	var quotes []map[string]any
	json.Unmarshal([]byte(quotesJSON), &quotes)
	resp := map[string]any{
		"recordedDate": recordedDate,
		"provider":     provider,
		"sourceType":   sourceType,
		"fetchedAt":    fetchedAt,
		"quotes":       quotes,
	}
	if displayDate.Valid { resp["displayDate"] = displayDate.String }
	if publishedAt.Valid { resp["publishedAt"] = publishedAt.String }
	if fallbackReason.Valid { resp["fallbackReason"] = fallbackReason.String }
	return resp
}

func ensureTodaysQuote(db *sql.DB) (map[string]any, error) {
	today := time.Now().UTC().Format("2006-01-02")

	// Check if we already have a fresh snapshot
	existing := getQuoteSnapshot(db, today)
	if existing != nil {
		// Consider it fresh if fetched within the last hour
		fetchedAt, _ := existing["fetchedAt"].(string)
		if fetchedAt != "" {
			t, err := time.Parse(time.RFC3339, fetchedAt)
			if err == nil && time.Since(t) < time.Hour {
				return existing, nil
			}
		}
	}

	// Fetch and store
	err := RefreshQuoteSnapshot(db, time.Now())
	if err != nil {
		return nil, err
	}

	return getQuoteSnapshot(db, today), nil
}

func StartQuoteScheduler(db *sql.DB, cfg *config.Config) {
	// Run immediately on startup
	go func() {
		runScheduledRefresh(db, "startup")
	}()

	// Schedule daily fetch
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day(),
				cfg.QuoteFetchHourUTC, cfg.QuoteFetchMinuteUTC, 0, 0, time.UTC)
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			<-time.After(time.Until(next))
			runScheduledRefresh(db, "daily")
		}
	}()
}

func runScheduledRefresh(db *sql.DB, trigger string) {
	today := time.Now().UTC().Format("2006-01-02")
	err := RefreshQuoteSnapshot(db, time.Now())
	if err != nil {
		log.Printf("[quotes] %s quote fetch failed: %v — retrying in 60s", trigger, err)
		time.AfterFunc(60*time.Second, func() {
			runScheduledRefresh(db, "retry")
		})
		return
	}
	log.Printf("[quotes] %s: stored latest quote snapshot for %s", trigger, today)
}
