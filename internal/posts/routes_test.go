package posts

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

func TestBlogSharePageAndEmbedImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openPostsTestDB(t)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO posts (id, title, content, author, shortDescription, thumbnail, tags, likes, comments, createdAt, updatedAt)
		VALUES ('post_123', 'A Gentle Test Post', '{}', 'Mira', 'A soft little description for Discord.', '', '["dev","notes"]', '[]', '[]', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	RegisterRoutes(router.Group("/"), &RouteConfig{
		DB:          db,
		WebsiteBase: "https://mirabellier.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/blog/post_123", nil)
	req.Header.Set("User-Agent", "Discordbot/2.0")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("crawler status = %d", resp.Code)
	}
	body := resp.Body.String()
	for _, expected := range []string{
		`property="og:type" content="article"`,
		`property="og:image"`,
		`property="og:image:width" content="1200"`,
		`property="og:image:height" content="630"`,
		`property="og:image:type" content="image/png"`,
		`name="twitter:card" content="summary_large_image"`,
		`property="article:tag" content="dev"`,
		`rel="canonical" href="https://mirabellier.com/blog/post_123"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("share HTML missing %q\n%s", expected, body)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/blog/post_123", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("browser status = %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/blog/post_123/embed-image.png?v=test", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assertPNGResponse(t, resp)
	if got := resp.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func openPostsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			avatar TEXT
		);
		CREATE TABLE posts (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			userId TEXT,
			author TEXT,
			shortDescription TEXT,
			thumbnail TEXT,
			tags TEXT,
			likes TEXT,
			comments TEXT,
			createdAt TEXT NOT NULL,
			updatedAt TEXT
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
