package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/embed"
	"github.com/mirabellier/mirabellier-backend-go/internal/seo"
	"golang.org/x/oauth2"
)

func RegisterRoutes(r *gin.RouterGroup, db *sql.DB, cfg *Config) {
	h := &handler{db: db, cfg: cfg}

	r.GET("/auth/discord", h.startDiscordOAuth)
	r.GET("/auth/discord/callback", h.discordOAuthCallback)

	authed := r.Group("")
	authed.Use(Middleware(db, cfg))
	{
		authed.GET("/me", h.getMe)
		authed.POST("/me", h.updateMe)
		authed.POST("/logout", h.logout)
	}

	r.GET("/user/:id", h.getUserByID)
	r.GET("/user/by-username/:username", h.getUserByUsername)
	r.GET("/user/:id/stats", h.getUserStats)
	r.GET("/profile/:username", h.profileSEOPage)
	r.GET("/profile-embed/:username", h.profileEmbedPNG)
	r.GET("/api/profile-embed/:username", h.profileEmbedPNG)
}

type handler struct {
	db  *sql.DB
	cfg *Config
}

type discordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Avatar     string `json:"avatar"`
	Banner     string `json:"banner"`
	GlobalName string `json:"global_name"`
}

func (h *handler) startDiscordOAuth(c *gin.Context) {
	redirectOrigin := c.Query("redirect_origin")
	if redirectOrigin == "" {
		redirectOrigin = c.GetHeader("Origin")
		if redirectOrigin == "" {
			redirectOrigin = c.GetHeader("Referer")
		}
	}
	if redirectOrigin == "" {
		redirectOrigin = h.cfg.FrontendURL
	}

	allowed := append([]string{h.cfg.FrontendURL}, h.cfg.FrontendURLs...)
	allowed = append(allowed, "http://localhost:5173", "http://127.0.0.1:5173")
	originOK := false
	for _, o := range allowed {
		if strings.TrimRight(o, "/") == strings.TrimRight(redirectOrigin, "/") {
			originOK = true
			break
		}
	}
	if !originOK {
		redirectOrigin = h.cfg.FrontendURL
	}

	c.SetCookie("mirabellier_oauth_origin", redirectOrigin, 600, "/", "", false, false)

	conf := h.discordOAuthConfig()
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOnline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *handler) discordOAuthCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code"})
		return
	}

	conf := h.discordOAuthConfig()
	tok, err := conf.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth exchange failed"})
		return
	}

	client := conf.Client(context.Background(), tok)
	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch Discord user"})
		return
	}
	defer resp.Body.Close()

	var dUser discordUser
	if err := json.NewDecoder(resp.Body).Decode(&dUser); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to parse Discord user"})
		return
	}

	user, err := FindOrCreateDiscordUser(h.db, dUser.ID, dUser.Username, dUser.GlobalName, dUser.Avatar, dUser.Banner, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
		return
	}

	sessionToken, err := GenerateToken(h.cfg.SessionSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation failed"})
		return
	}

	_, err = h.db.Exec("INSERT INTO sessions (token, userId) VALUES (?, ?)", sessionToken, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session creation failed"})
		return
	}

	SetSessionCookie(c, sessionToken, h.cfg.SessionCookieName, h.cfg.SessionCookieMaxAgeSeconds, h.cfg.SessionCookieSecure)

	origin, _ := c.Cookie("mirabellier_oauth_origin")
	if origin == "" {
		origin = h.cfg.FrontendURL
	}
	c.SetCookie("mirabellier_oauth_origin", "", -1, "/", "", false, false)
	c.Redirect(http.StatusTemporaryRedirect, origin+"/auth/callback")
}

func (h *handler) getMe(c *gin.Context) {
	user := GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *handler) updateMe(c *gin.Context) {
	user := GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	bio := strings.TrimSpace(c.PostForm("bio"))
	location := strings.TrimSpace(c.PostForm("location"))
	website := strings.TrimSpace(c.PostForm("website"))

	if username != "" {
		_, err := h.db.Exec(`UPDATE users SET username = ?, bio = ?, location = ?, website = ? WHERE id = ?`,
			username, bio, location, website, user.ID)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username taken or update failed"})
			return
		}
	}

	updated, _ := GetUserByID(h.db, user.ID, h.cfg)
	c.JSON(http.StatusOK, updated)
}

func (h *handler) logout(c *gin.Context) {
	user := GetUser(c)
	if user != nil {
		h.db.Exec("DELETE FROM sessions WHERE userId = ?", user.ID)
	}
	ClearSessionCookie(c, h.cfg.SessionCookieName)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *handler) getUserByID(c *gin.Context) {
	user, err := GetUserByID(h.db, c.Param("id"), h.cfg)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, ToUserPublic(user))
}

func (h *handler) getUserByUsername(c *gin.Context) {
	user, err := GetUserByUsername(h.db, c.Param("username"), h.cfg)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, ToUserPublic(user))
}

func (h *handler) getUserStats(c *gin.Context) {
	id := c.Param("id")
	stats := UserStats{}

	h.db.QueryRow("SELECT COUNT(*) FROM posts WHERE userId = ?", id).Scan(&stats.PostsCount)
	h.db.QueryRow("SELECT COUNT(*) FROM likes WHERE identity_type = 'user' AND identity_key = ?", id).Scan(&stats.LikesCount)
	h.db.QueryRow("SELECT COUNT(*) FROM comments WHERE userId = ?", id).Scan(&stats.CommentsCount)

	rows, err := h.db.Query("SELECT id, title, createdAt FROM posts WHERE userId = ? ORDER BY createdAt DESC LIMIT 5", id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p PostSummary
			rows.Scan(&p.ID, &p.Title, &p.CreatedAt)
			stats.RecentPosts = append(stats.RecentPosts, p)
		}
	}

	c.JSON(http.StatusOK, stats)
}

func (h *handler) profileSEOPage(c *gin.Context) {
	username := c.Param("username")
	user, err := GetUserByUsername(h.db, username, h.cfg)
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}
	if !seo.IsCrawler(c.GetHeader("User-Agent")) {
		c.Redirect(http.StatusTemporaryRedirect, strings.TrimRight(h.cfg.FrontendURL, "/")+"/profile/"+username)
		return
	}
	desc := "A Mirabellier profile."
	if user.Bio != nil && strings.TrimSpace(*user.Bio) != "" {
		desc = *user.Bio
	}
	version := seo.VersionHash("profile-render-v1", user.Username, stringValue(user.Avatar), stringValue(user.Bio), user.CreatedAt)
	seo.RenderShareHTML(c, h.cfg.WebsiteBase, seo.SharePage{
		Title:       user.Username + "'s Profile",
		Description: desc,
		Path:        "/profile/" + username,
		ImagePath:   "/profile-embed/" + username + ".png?v=" + version,
		ImageWidth:  embed.PreviewWidth,
		ImageHeight: embed.PreviewHeight,
		ImageAlt:    "A preview image of " + user.Username + "'s Mirabellier profile.",
	})
}

func (h *handler) profileEmbedPNG(c *gin.Context) {
	user, err := GetUserByUsername(h.db, profileEmbedUsername(c.Param("username")), h.cfg)
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}
	avatar := user.Avatar
	if avatar != nil {
		absoluteAvatar := seo.PublicURL(h.cfg.WebsiteBase, *avatar)
		avatar = &absoluteAvatar
	}
	png, err := embed.RenderProfileEmbed(user.Username, avatar, user.Bio)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to render profile preview image")
		return
	}
	c.Header("Content-Type", "image/png")
	c.Header("Content-Length", strconv.Itoa(len(png)))
	seo.SetEmbedImageCacheHeaders(c)
	c.Data(http.StatusOK, "image/png", png)
}

func profileEmbedUsername(value string) string {
	return strings.TrimSuffix(value, ".png")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (h *handler) discordOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     h.cfg.DiscordClientID,
		ClientSecret: h.cfg.DiscordClientSecret,
		RedirectURL:  h.cfg.DiscordCallbackURL,
		Scopes:       []string{"identify"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/api/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}
}
