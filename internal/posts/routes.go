package posts

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/auth"
	"github.com/mirabellier/mirabellier-backend-go/internal/embed"
	"github.com/mirabellier/mirabellier-backend-go/internal/seo"
)

type RouteConfig struct {
	DB          *sql.DB
	WebsiteBase string
}

func RegisterRoutes(r *gin.RouterGroup, cfg *RouteConfig) {
	h := &handler{db: cfg.DB, websiteBase: cfg.WebsiteBase}

	r.GET("/posts", h.listPosts)
	r.GET("/posts/:id", h.getPost)
	r.POST("/posts", h.createPost)
	r.PUT("/posts/:id", h.updatePost)
	r.DELETE("/posts/:id", h.deletePost)
	r.POST("/posts/:id/like", h.toggleLike)
	r.POST("/posts/:id/comments", auth.Require(), h.addComment)
	r.GET("/tags", h.listTags)
	r.GET("/blog/:id", h.blogSEOPage)
	r.GET("/blog/:id/embed-image.png", h.blogEmbedImage)
}

type handler struct {
	db          *sql.DB
	websiteBase string
}

func (h *handler) blogSEOPage(c *gin.Context) {
	post := h.loadBlogPost(c)
	if post == nil {
		return
	}

	if !seo.IsCrawler(c.GetHeader("User-Agent")) {
		c.Redirect(http.StatusTemporaryRedirect, strings.TrimRight(h.websiteBase, "/")+"/blog/"+post.ID)
		return
	}

	h.renderBlogShareHTML(c, post)
}

func (h *handler) blogEmbedImage(c *gin.Context) {
	post := h.loadBlogPost(c)
	if post == nil {
		return
	}
	png, err := embed.RenderBlogEmbed(blogPreviewFromPost(post))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to render blog preview image")
		return
	}
	c.Header("Content-Type", "image/png")
	c.Header("Content-Length", strconv.Itoa(len(png)))
	seo.SetEmbedImageCacheHeaders(c)
	c.Data(http.StatusOK, "image/png", png)
}

func (h *handler) loadBlogPost(c *gin.Context) *Post {
	id := c.Param("id")
	if strings.Contains(id, "-") {
		parts := strings.Split(id, "-")
		id = parts[len(parts)-1]
	}

	post, err := GetPost(h.db, id)
	if err != nil {
		c.String(http.StatusNotFound, "Post not found")
		return nil
	}
	return post
}

func (h *handler) renderBlogShareHTML(c *gin.Context, post *Post) {
	base := seo.WebsiteBase(h.websiteBase)
	canonicalURL := base + "/blog/" + post.ID
	version := seo.VersionHash("blog-render-v1", post.ID, post.Title, post.CreatedAt, stringValue(post.UpdatedAt), stringValue(post.ShortDescription), stringValue(post.Thumbnail))
	imageURL := canonicalURL + "/embed-image.png?v=" + version
	description := strings.TrimSpace(stringValue(post.ShortDescription))
	if description == "" {
		description = "A post from Mirabellier."
	}
	author := strings.TrimSpace(post.Author)
	if author == "" {
		author = "Mirabellier"
	}
	jsonLD, _ := json.Marshal(map[string]any{
		"@context":         "https://schema.org",
		"@type":            "BlogPosting",
		"headline":         post.Title,
		"description":      description,
		"url":              canonicalURL,
		"mainEntityOfPage": canonicalURL,
		"datePublished":    post.CreatedAt,
		"dateModified":     firstNonEmpty(stringValue(post.UpdatedAt), post.CreatedAt),
		"author": map[string]any{
			"@type": "Person",
			"name":  author,
		},
		"publisher": map[string]any{
			"@type": "Person",
			"name":  "Mirabellier",
			"url":   base + "/",
		},
		"image": []string{imageURL},
	})
	escapedJSON := strings.NewReplacer("<", "\\u003c", ">", "\\u003e", "&", "\\u0026").Replace(string(jsonLD))
	tagMeta := ""
	for _, tag := range post.Tags {
		tagMeta += fmt.Sprintf("\n<meta property=\"article:tag\" content=\"%s\">", html.EscapeString(tag))
	}
	updatedAt := firstNonEmpty(stringValue(post.UpdatedAt), post.CreatedAt)
	body := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<meta name="description" content="%s">
<meta name="robots" content="index,follow,max-image-preview:large,max-snippet:-1,max-video-preview:-1">
<meta property="og:type" content="article">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:site_name" content="Mirabellier">
<meta property="og:url" content="%s">
<meta property="og:image" content="%s">
<meta property="og:image:width" content="%d">
<meta property="og:image:height" content="%d">
<meta property="og:image:type" content="image/png">
<meta property="og:image:alt" content="%s">
<meta property="article:published_time" content="%s">
<meta property="article:modified_time" content="%s">
<meta property="article:author" content="%s">%s
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
<meta name="twitter:image" content="%s">
<meta name="twitter:image:alt" content="%s">
<link rel="canonical" href="%s">
<script type="application/ld+json">%s</script>
</head>
<body><main><h1>%s</h1><p>%s</p><p><a href="%s">Read full post</a></p></main></body>
</html>`,
		html.EscapeString(post.Title), html.EscapeString(description),
		html.EscapeString(post.Title), html.EscapeString(description), html.EscapeString(canonicalURL),
		html.EscapeString(imageURL), embed.PreviewWidth, embed.PreviewHeight, html.EscapeString(post.Title),
		html.EscapeString(post.CreatedAt), html.EscapeString(updatedAt), html.EscapeString(author), tagMeta,
		html.EscapeString(post.Title), html.EscapeString(description), html.EscapeString(imageURL),
		html.EscapeString(post.Title), html.EscapeString(canonicalURL), escapedJSON,
		html.EscapeString(post.Title), html.EscapeString(description), html.EscapeString(canonicalURL))

	c.Header("Content-Type", "text/html; charset=utf-8")
	seo.SetNoStoreHeaders(c)
	c.String(http.StatusOK, body)
}

func blogPreviewFromPost(post *Post) embed.BlogPreview {
	return embed.BlogPreview{
		Title:       post.Title,
		Description: stringValue(post.ShortDescription),
		Author:      post.Author,
		PublishedAt: post.CreatedAt,
		UpdatedAt:   stringValue(post.UpdatedAt),
		Thumbnail:   stringValue(post.Thumbnail),
		Tags:        post.Tags,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (h *handler) listPosts(c *gin.Context) {
	posts, err := ListPosts(h.db)
	if err != nil {
		log.Printf("[posts] list error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
	}
	if posts == nil {
		posts = []Post{}
	}
	c.JSON(http.StatusOK, posts)
}

func (h *handler) getPost(c *gin.Context) {
	id := c.Param("id")
	if strings.Contains(id, "-") {
		parts := strings.Split(id, "-")
		id = parts[len(parts)-1]
	}
	post, err := GetPost(h.db, id)
	if err != nil {
		log.Printf("[posts] getPost(%s) error: %v", id, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}
	c.JSON(http.StatusOK, post)
}

func (h *handler) createPost(c *gin.Context) {
	var input CreatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user := auth.GetUser(c)
	if user != nil {
		input.UserID = &user.ID
	}

	id := generateID("post")
	post, err := CreatePost(h.db, &input, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	c.JSON(http.StatusCreated, post)
}

func (h *handler) updatePost(c *gin.Context) {
	id := c.Param("id")
	var input UpdatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user := auth.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	post, err := UpdatePost(h.db, id, &input, user.ID)
	if err != nil {
		if err.Error() == "not authorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	c.JSON(http.StatusOK, post)
}

func (h *handler) deletePost(c *gin.Context) {
	id := c.Param("id")

	user := auth.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	if err := DeletePost(h.db, id, user.ID); err != nil {
		if err.Error() == "not authorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *handler) toggleLike(c *gin.Context) {
	postID := c.Param("id")

	var action LikeAction
	if err := c.ShouldBindJSON(&action); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user := auth.GetUser(c)
	var identityType, identityKey string

	if user != nil {
		identityType = "user"
		identityKey = user.ID
	} else {
		identityType = "anonymous"
		// Validate anonymous ID pattern: anon:[a-z0-9-]{12,} (Node.js behavior)
		anonIDPattern := regexp.MustCompile(`^anon:[a-z0-9-]{12,}$`)
		anonID := c.GetHeader("X-Like-Anonymous-Id")
		if anonIDPattern.MatchString(strings.ToLower(anonID)) {
			identityKey = anonID
		} else {
			clientID := c.GetHeader("X-Like-Client-Id")
			if anonIDPattern.MatchString(strings.ToLower(clientID)) {
				identityKey = clientID
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing like identity"})
				return
			}
		}
	}

	result, err := ToggleLike(h.db, postID, identityType, identityKey, action.Action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle like"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *handler) addComment(c *gin.Context) {
	postID := c.Param("id")

	var input CreateCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user := auth.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	comment, err := AddComment(h.db, postID, &input, &user.ID, &user.Username, user.Avatar)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Node.js returns { ...comment, user: userPublic(user), children: [] }
	comment.Children = []Comment{}

	c.JSON(http.StatusCreated, comment)
}

func (h *handler) listTags(c *gin.Context) {
	tags, err := ListTags(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tags"})
		return
	}
	if tags == nil {
		tags = []string{}
	}
	c.JSON(http.StatusOK, tags)
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixMilli(), rand.Intn(10000))
}
