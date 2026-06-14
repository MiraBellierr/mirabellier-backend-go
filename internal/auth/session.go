package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// --- Token ---

// GenerateToken creates a random token with HMAC signature: {randomHex}.{signatureHex}
func GenerateToken(secret string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	randomPart := hex.EncodeToString(raw)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(randomPart))
	signature := hex.EncodeToString(mac.Sum(nil))

	return randomPart + "." + signature, nil
}

// ValidateTokenSignature checks the HMAC signature on a token.
func ValidateTokenSignature(token, secret string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}

// --- Cookie ---

// SetSessionCookie writes the httpOnly session cookie.
func SetSessionCookie(c *gin.Context, token string, cookieName string, maxAge int, secure bool) {
	if proto := c.GetHeader("X-Forwarded-Proto"); proto == "https" {
		secure = true
	}

	c.SetCookie(cookieName, token, maxAge, "/", "", secure, true)
	c.SetSameSite(http.SameSiteLaxMode)
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(c *gin.Context, cookieName string) {
	c.SetCookie(cookieName, "", -1, "/", "", false, true)
}
