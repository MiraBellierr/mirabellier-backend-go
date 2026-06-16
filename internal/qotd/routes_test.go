package qotd

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func TestQOTDSharePageAndEmbedImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openQOTDTestDB(t)
	today := time.Now().UTC().Format("2006-01-02")
	_, err := db.Exec(`INSERT INTO daily_questions (recordedDate, prompt, createdAt, updatedAt) VALUES (?, ?, ?, ?)`,
		today, "What small joy are you carrying today?", time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	RegisterRoutes(router.Group("/"), db, &Config{
		WebsiteBase: "https://mirabellier.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/question-of-the-day", nil)
	req.Header.Set("User-Agent", "Discordbot/2.0")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("crawler status = %d", resp.Code)
	}
	body := resp.Body.String()
	for _, expected := range []string{
		`property="og:image"`,
		`name="twitter:card" content="summary_large_image"`,
		`property="og:image:width" content="1200"`,
		`property="og:image:height" content="630"`,
		`rel="canonical" href="https://mirabellier.com/question-of-the-day"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("share HTML missing %q\n%s", expected, body)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/question-of-the-day", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("browser status = %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/question-of-the-day/embed-image.png?v=test", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assertPNGResponse(t, resp)
	if got := resp.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func openQOTDTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE daily_questions (
			recordedDate TEXT PRIMARY KEY,
			prompt TEXT NOT NULL,
			lockedAt TEXT,
			archivedAt TEXT,
			createdAt TEXT NOT NULL,
			updatedAt TEXT NOT NULL
		);
		CREATE TABLE daily_question_answers (
			id TEXT PRIMARY KEY,
			recordedDate TEXT NOT NULL,
			createdAt TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertPNGResponse(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("png status = %d", resp.Code)
	}
	if got := resp.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q", got)
	}
	body := resp.Body.Bytes()
	if len(body) < 8 || string(body[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("response is not a PNG, len=%d", len(body))
	}
}
