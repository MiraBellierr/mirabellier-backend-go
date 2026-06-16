package auth

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

func TestProfileSharePageAndEmbedImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			discordId TEXT,
			avatar TEXT,
			banner TEXT,
			bio TEXT,
			location TEXT,
			website TEXT,
			createdAt TEXT NOT NULL
		);
		CREATE TABLE sessions (token TEXT PRIMARY KEY, userId TEXT NOT NULL);
		INSERT INTO users (id, username, bio, createdAt) VALUES ('user_1', 'mira', 'hello from profile', ?);
	`, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	RegisterRoutes(router.Group("/"), db, &Config{
		FrontendURL: "http://localhost:5173",
		WebsiteBase: "https://mirabellier.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/profile/mira", nil)
	req.Header.Set("User-Agent", "Discordbot/2.0")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("crawler status = %d", resp.Code)
	}
	body := resp.Body.String()
	for _, expected := range []string{
		`property="og:image"`,
		`property="og:image" content="https://mirabellier.com/profile-embed/mira.png?v=`,
		`property="og:image:secure_url" content="https://mirabellier.com/profile-embed/mira.png?v=`,
		`property="og:image:type" content="image/png"`,
		`name="twitter:card" content="summary_large_image"`,
		`name="twitter:image:src" content="https://mirabellier.com/profile-embed/mira.png?v=`,
		`rel="canonical" href="https://mirabellier.com/profile/mira"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("share HTML missing %q\n%s", expected, body)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/profile-embed/mira.png?v=test", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assertPNGResponse(t, resp)
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
