package quotes

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/config"
	_ "modernc.org/sqlite"
)

func TestQuotesSharePageAndEmbedImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openQuotesTestDB(t)
	today := time.Now().UTC().Format("2006-01-02")
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO quote_snapshots (recordedDate, provider, sourceType, fetchedAt, quotesJson, createdAt, updatedAt)
		VALUES (?, 'brainyquote', 'rss', ?, ?, ?, ?)
	`, today, now, `[{"category":"Quote of the Day","quote":"Stay gentle with the work.","author":"Mira"}]`, now, now)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	RegisterRoutes(router.Group("/"), db, &config.Config{
		FrontendURL: "http://localhost:5173",
		WebsiteBase: "https://mirabellier.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/quotes", nil)
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
		`property="og:image:type" content="image/png"`,
		`rel="canonical" href="https://mirabellier.com/quotes"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("share HTML missing %q\n%s", expected, body)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/quotes", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("browser status = %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/quotes/embed-image.png?v=test", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assertPNGResponse(t, resp)
	if got := resp.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func openQuotesTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE quote_snapshots (
			recordedDate TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			sourceType TEXT NOT NULL,
			displayDate TEXT,
			publishedAt TEXT,
			fetchedAt TEXT NOT NULL,
			fallbackReason TEXT,
			quotesJson TEXT NOT NULL,
			createdAt TEXT NOT NULL,
			updatedAt TEXT NOT NULL
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
