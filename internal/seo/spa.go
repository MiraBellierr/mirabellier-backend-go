package seo

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var crawlerPatterns = []string{
	"bot", "spider", "crawler", "scraper",
	"facebook", "twitter", "discord", "slack", "telegram", "whatsapp", "linkedin",
	"google", "bing", "yahoo", "duckduck", "baidu", "yandex",
	"applebot", "pinterest", "reddit", "tumblr", "snapchat",
}

func IsCrawler(ua string) bool {
	ua = strings.ToLower(ua)
	for _, pattern := range crawlerPatterns {
		if strings.Contains(ua, pattern) {
			return true
		}
	}
	return false
}

func HandleSpaRequest(c *gin.Context, spaPath string, frontendURL string) {
	ua := c.GetHeader("User-Agent")

	if IsCrawler(ua) {
		spaFile := resolveSpaFile(spaPath)
		if spaFile != "" {
			c.File(spaFile)
			return
		}
	}

	redirectURL := frontendURL + c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		redirectURL += "?" + c.Request.URL.RawQuery
	}
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func resolveSpaFile(spaPath string) string {
	if spaPath != "" {
		if stat, err := os.Stat(spaPath); err == nil && !stat.IsDir() {
			return spaPath
		}
	}
	candidates := []string{
		"/var/www/mirabellier/dist/index.html",
		"/var/www/mirabellier.com/current/index.html",
		"../../dist/index.html",
	}
	for _, path := range candidates {
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			return path
		}
	}
	return ""
}
