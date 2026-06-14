package posts

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/auth"
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
}

type handler struct {
	db          *sql.DB
	websiteBase string
}

func (h *handler) blogSEOPage(c *gin.Context) {
	id := c.Param("id")
	if strings.Contains(id, "-") {
		parts := strings.Split(id, "-")
		id = parts[len(parts)-1]
	}

	post, err := GetPost(h.db, id)
	if err != nil {
		c.String(http.StatusNotFound, "Post not found")
		return
	}

	seoData := &seo.BlogPostData{
		ID:               post.ID,
		Title:            post.Title,
		ShortDescription: post.ShortDescription,
		Thumbnail:        post.Thumbnail,
	}
	seo.RenderBlogSEOPage(c, seoData, h.websiteBase)
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
