package seo

import (
	"database/sql"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BlogPostData is the minimal post data needed for SEO page rendering.
type BlogPostData struct {
	ID               string
	Title            string
	ShortDescription *string
	Thumbnail        *string
}

// RenderBlogSEOPage serves an OG-tagged HTML page for blog post previews (crawlers).
func RenderBlogSEOPage(c *gin.Context, post *BlogPostData, websiteBase string) {
	ua := c.GetHeader("User-Agent")
	if !IsCrawler(ua) {
		c.Redirect(http.StatusTemporaryRedirect, websiteBase+"/blog/"+post.ID)
		return
	}

	title := html.EscapeString(post.Title)
	desc := ""
	if post.ShortDescription != nil {
		desc = html.EscapeString(*post.ShortDescription)
	}
	thumb := ""
	if post.Thumbnail != nil {
		thumb = html.EscapeString(*post.Thumbnail)
	}

	id := post.ID

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s — Mirabellier</title>
<meta name="description" content="%s">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:type" content="article">
<meta property="og:url" content="%s/blog/%s">
<meta property="og:site_name" content="Mirabellier">
<meta property="og:image" content="%s">
<meta name="twitter:card" content="summary_large_image">
</head>
<body><h1>%s</h1></body>
</html>`, title, desc, title, desc, websiteBase, id, thumb, title)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, pageHTML)
}

// RenderProfileSEOPage serves an OG-tagged HTML page for user profile previews.
func RenderProfileSEOPage(c *gin.Context, db *sql.DB, websiteBase string) {
	username := c.Param("username")

	ua := c.GetHeader("User-Agent")
	if !IsCrawler(ua) {
		c.Redirect(http.StatusTemporaryRedirect, websiteBase+"/profile/"+username)
		return
	}

	var userID, displayName string
	var avatar, bio *string
	err := db.QueryRow("SELECT id, username, avatar, bio FROM users WHERE username = ?", username).
		Scan(&userID, &displayName, &avatar, &bio)
	if err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}

	name := html.EscapeString(displayName)
	desc := ""
	if bio != nil && *bio != "" {
		desc = html.EscapeString(*bio)
	}
	avatarURL := ""
	if avatar != nil {
		avatarURL = html.EscapeString(*avatar)
	}

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s — Mirabellier</title>
<meta name="description" content="%s">
<meta property="og:title" content="%s's Profile">
<meta property="og:description" content="%s">
<meta property="og:type" content="profile">
<meta property="og:url" content="%s/profile/%s">
<meta property="og:site_name" content="Mirabellier">
<meta property="og:image" content="%s">
<meta name="twitter:card" content="summary">
</head>
<body><h1>%s</h1></body>
</html>`, name, desc, name, desc, websiteBase, username, avatarURL, name)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, pageHTML)
}

// RenderShrineSEOPage serves an OG-tagged HTML page for shrine previews.
func RenderShrineSEOPage(c *gin.Context, db *sql.DB, websiteBase string) {
	slug := c.Param("slug")

	ua := c.GetHeader("User-Agent")
	if !IsCrawler(ua) {
		if slug == "" {
			c.Redirect(http.StatusTemporaryRedirect, websiteBase+"/shrine")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, websiteBase+"/shrine/"+slug)
		}
		return
	}

	if slug == "" {
		pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Character Shrines — Mirabellier</title>
<meta property="og:title" content="Character Shrines — Mirabellier">
<meta property="og:type" content="website">
<meta property="og:url" content="%s/shrine">
<meta property="og:site_name" content="Mirabellier">
</head>
<body><h1>Character Shrines</h1></body>
</html>`, websiteBase)

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, pageHTML)
		return
	}

	var title string
	var description, image *string
	err := db.QueryRow("SELECT title, description, image FROM shrine_pages WHERE slug = ?", slug).
		Scan(&title, &description, &image)
	if err != nil {
		title = strings.Title(strings.ReplaceAll(slug, "-", " ")) + " Shrine"
	}

	name := html.EscapeString(title)
	desc := ""
	if description != nil {
		desc = html.EscapeString(*description)
	}
	imgURL := ""
	if image != nil {
		imgURL = html.EscapeString(*image)
	}

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s — Mirabellier</title>
<meta name="description" content="%s">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:type" content="website">
<meta property="og:url" content="%s/shrine/%s">
<meta property="og:site_name" content="Mirabellier">
<meta property="og:image" content="%s">
<meta name="twitter:card" content="summary_large_image">
</head>
<body><h1>%s</h1></body>
</html>`, name, desc, name, desc, websiteBase, slug, imgURL, name)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, pageHTML)
}

// RenderQOTDSEOPage serves OG-tagged HTML for QOTD.
func RenderQOTDSEOPage(c *gin.Context, db *sql.DB, websiteBase string) {
	ua := c.GetHeader("User-Agent")
	if !IsCrawler(ua) {
		c.Redirect(http.StatusTemporaryRedirect, websiteBase+"/question-of-the-day")
		return
	}

	prompt := "Question of the Day"
	db.QueryRow("SELECT prompt FROM daily_questions WHERE recorded_date = date('now') AND archived_at IS NULL").Scan(&prompt)

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Question of the Day — Mirabellier</title>
<meta name="description" content="%s">
<meta property="og:title" content="Question of the Day — Mirabellier">
<meta property="og:description" content="%s">
<meta property="og:type" content="website">
<meta property="og:url" content="%s/question-of-the-day">
<meta property="og:site_name" content="Mirabellier">
<meta name="twitter:card" content="summary">
</head>
<body><h1>%s</h1></body>
</html>`, html.EscapeString(prompt), html.EscapeString(prompt), websiteBase, html.EscapeString(prompt))

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, pageHTML)
}

// RenderQuotesSEOPage serves OG-tagged HTML for quotes page.
func RenderQuotesSEOPage(c *gin.Context, db *sql.DB, websiteBase string) {
	ua := c.GetHeader("User-Agent")
	if !IsCrawler(ua) {
		c.Redirect(http.StatusTemporaryRedirect, websiteBase+"/quotes")
		return
	}

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Quote of the Day — Mirabellier</title>
<meta property="og:title" content="Quote of the Day — Mirabellier">
<meta property="og:type" content="website">
<meta property="og:url" content="%s/quotes">
<meta property="og:site_name" content="Mirabellier">
<meta name="twitter:card" content="summary">
</head>
<body><h1>Quote of the Day</h1></body>
</html>`, websiteBase)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, pageHTML)
}

// RenderAnimeSEOPage serves OG-tagged HTML for anime page.
func RenderAnimeSEOPage(c *gin.Context, db *sql.DB, websiteBase string) {
	ua := c.GetHeader("User-Agent")
	if !IsCrawler(ua) {
		c.Redirect(http.StatusTemporaryRedirect, websiteBase+"/anime")
		return
	}

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Currently Watching — Mirabellier</title>
<meta property="og:title" content="Currently Watching — Mirabellier">
<meta property="og:type" content="website">
<meta property="og:url" content="%s/anime">
<meta property="og:site_name" content="Mirabellier">
<meta name="twitter:card" content="summary_large_image">
</head>
<body><h1>Currently Watching</h1></body>
</html>`, websiteBase)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, pageHTML)
}
