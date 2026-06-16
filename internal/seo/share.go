package seo

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type SharePage struct {
	Title       string
	Description string
	Path        string
	ImagePath   string
	ImageWidth  int
	ImageHeight int
	ImageAlt    string
}

func WebsiteBase(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return "https://mirabellier.com"
	}
	return base
}

func PublicURL(base, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	if strings.HasPrefix(value, "/") {
		return WebsiteBase(base) + value
	}
	return WebsiteBase(base) + "/" + strings.TrimLeft(value, "/")
}

func SetNoStoreHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Surrogate-Control", "no-store")
}

func SetEmbedImageCacheHeaders(c *gin.Context) {
	if strings.TrimSpace(c.Query("v")) != "" {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
}

func VersionHash(parts ...string) string {
	h := sha1.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func RenderShareHTML(c *gin.Context, websiteBase string, page SharePage) {
	base := WebsiteBase(websiteBase)
	path := "/" + strings.TrimLeft(page.Path, "/")
	imagePath := "/" + strings.TrimLeft(page.ImagePath, "/")
	canonicalURL := base + path
	imageURL := base + imagePath

	jsonLD, _ := json.Marshal(map[string]any{
		"@context":         "https://schema.org",
		"@type":            "CollectionPage",
		"name":             page.Title,
		"url":              canonicalURL,
		"mainEntityOfPage": canonicalURL,
		"image":            []string{imageURL},
		"isPartOf": map[string]any{
			"@type": "WebSite",
			"name":  "Mirabellier",
			"url":   base + "/",
		},
	})

	escapedJSON := strings.NewReplacer("<", "\\u003c", ">", "\\u003e", "&", "\\u0026").Replace(string(jsonLD))
	title := html.EscapeString(page.Title)
	desc := html.EscapeString(page.Description)
	imgAlt := html.EscapeString(page.ImageAlt)
	htmlBody := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<meta name="description" content="%s">
<meta name="robots" content="index,follow,max-image-preview:large,max-snippet:-1,max-video-preview:-1">
<meta property="og:type" content="website">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:site_name" content="Mirabellier">
<meta property="og:url" content="%s">
<meta property="og:image" content="%s">
<meta property="og:image:width" content="%d">
<meta property="og:image:height" content="%d">
<meta property="og:image:type" content="image/png">
<meta property="og:image:alt" content="%s">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
<meta name="twitter:image" content="%s">
<meta name="twitter:image:alt" content="%s">
<link rel="canonical" href="%s">
<script type="application/ld+json">%s</script>
</head>
<body><main><h1>%s</h1><p><a href="%s">Open this page</a></p></main></body>
</html>`,
		title, desc, title, desc, html.EscapeString(canonicalURL), html.EscapeString(imageURL),
		page.ImageWidth, page.ImageHeight, imgAlt, title, desc, html.EscapeString(imageURL),
		imgAlt, html.EscapeString(canonicalURL), escapedJSON, title, html.EscapeString(canonicalURL))

	c.Header("Content-Type", "text/html; charset=utf-8")
	SetNoStoreHeaders(c)
	c.String(http.StatusOK, htmlBody)
}
