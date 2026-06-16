package auth

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

const userContextKey = "mirabellier_user"

// Middleware resolves the authenticated user from cookie or Authorization header.
// Does not block — handlers check GetUser for nil.
func Middleware(db *sql.DB, cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		if authHeader := c.GetHeader("Authorization"); len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}

		if token == "" {
			if cookieToken, err := c.Cookie(cfg.SessionCookieName); err == nil {
				token = cookieToken
			}
		}

		if token != "" {
			user, err := GetUserByToken(db, token, cfg)
			if err == nil {
				c.Set(userContextKey, user)
			}
		}

		c.Next()
	}
}

// Require blocks requests without a valid user.
func Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetUser(c) == nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "Authentication required"})
			return
		}
		c.Next()
	}
}

// GetUser retrieves the authenticated user from context.
func GetUser(c *gin.Context) *User {
	if user, ok := c.Get(userContextKey); ok {
		if u, ok := user.(*User); ok {
			return u
		}
	}
	return nil
}

// IsOwner checks if the user is an owner/admin.
func IsOwner(user *User, cfg *Config) bool {
	if user == nil || user.DiscordID == nil {
		return false
	}
	for _, id := range cfg.OwnerDiscordIDs {
		if *user.DiscordID == id {
			return true
		}
	}
	return false
}

// Config is the subset of app config needed by the auth package.
type Config struct {
	SessionSecret              string
	SessionCookieName          string
	SessionCookieMaxAgeSeconds int
	SessionCookieSecure        bool
	OwnerDiscordIDs            []string
	DiscordClientID            string
	DiscordClientSecret        string
	DiscordCallbackURL         string
	FrontendURL                string
	FrontendURLs               []string
	WebsiteBase                string
}
