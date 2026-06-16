package shrines

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mirabellier/mirabellier-backend-go/internal/auth"
	"github.com/mirabellier/mirabellier-backend-go/internal/embed"
	"github.com/mirabellier/mirabellier-backend-go/internal/seo"
)

type ShrinePage struct {
	Slug        string          `json:"slug"`
	Path        string          `json:"path"`
	Title       string          `json:"title"`
	Description *string         `json:"description,omitempty"`
	Excerpt     *string         `json:"excerpt,omitempty"`
	Image       *string         `json:"image,omitempty"`
	ImageAlt    *string         `json:"imageAlt,omitempty"`
	SchemaType  string          `json:"schemaType"`
	About       json.RawMessage `json:"about,omitempty"`
	Keywords    json.RawMessage `json:"keywords,omitempty"`
	CTALabel    *string         `json:"ctaLabel,omitempty"`
	Priority    *string         `json:"priority,omitempty"`
	Changefreq  *string         `json:"changefreq,omitempty"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

var hardcodedShrines = map[string]ShrinePage{
	"kanna": {
		Slug:  "kanna",
		Path:  "/shrine/kanna",
		Title: "Kanna Shrine",
	},
	"rossina": {
		Slug:  "rossina",
		Path:  "/shrine/rossina",
		Title: "Rossina Shrine",
	},
}

type Config struct {
	FrontendURL string
	WebsiteBase string
}

func RegisterRoutes(r *gin.RouterGroup, db *sql.DB, cfg *Config) {
	h := &handler{db: db, cfg: cfg}

	r.GET("/shrines/pages", h.listPages)
	r.GET("/shrines/pages/:slug", h.getPage)
	r.POST("/shrines/pages", auth.Require(), h.createPage)
	r.PUT("/shrines/pages/:slug", auth.Require(), h.updatePage)
	r.GET("/shrine", h.hubSEOPage)
	r.GET("/shrine/embed-image.png", h.hubEmbedImage)
	r.GET("/shrine/:slug/embed-image.png", h.shrineEmbedImage)
	r.GET("/shrine/:slug", h.shrineSEOPage)
}

type handler struct {
	db  *sql.DB
	cfg *Config
}

func (h *handler) listPages(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT slug, path, title, description, excerpt, image, imageAlt, schemaType,
		       aboutJson, keywordsJson, ctaLabel, priority, changefreq, payloadJson,
		       createdAt, updatedAt
		FROM shrine_pages ORDER BY createdAt DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch shrine pages"})
		return
	}
	defer rows.Close()

	var pages []ShrinePage
	for rows.Next() {
		var p ShrinePage
		var aboutJSON, keywordsJSON, payloadJSON string
		rows.Scan(&p.Slug, &p.Path, &p.Title, &p.Description, &p.Excerpt, &p.Image, &p.ImageAlt,
			&p.SchemaType, &aboutJSON, &keywordsJSON, &p.CTALabel, &p.Priority, &p.Changefreq,
			&payloadJSON, &p.CreatedAt, &p.UpdatedAt)

		p.About = json.RawMessage(aboutJSON)
		p.Keywords = json.RawMessage(keywordsJSON)
		p.Payload = json.RawMessage(payloadJSON)
		pages = append(pages, p)
	}
	if pages == nil {
		pages = []ShrinePage{}
	}

	c.JSON(http.StatusOK, pages)
}

func (h *handler) getPage(c *gin.Context) {
	slug := c.Param("slug")

	var p ShrinePage
	var aboutJSON, keywordsJSON, payloadJSON string
	err := h.db.QueryRow(`
		SELECT slug, path, title, description, excerpt, image, imageAlt, schemaType,
		       aboutJson, keywordsJson, ctaLabel, priority, changefreq, payloadJson,
		       createdAt, updatedAt
		FROM shrine_pages WHERE slug = ?
	`, slug).Scan(&p.Slug, &p.Path, &p.Title, &p.Description, &p.Excerpt, &p.Image, &p.ImageAlt,
		&p.SchemaType, &aboutJSON, &keywordsJSON, &p.CTALabel, &p.Priority, &p.Changefreq,
		&payloadJSON, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shrine page not found"})
		return
	}
	p.About = json.RawMessage(aboutJSON)
	p.Keywords = json.RawMessage(keywordsJSON)
	p.Payload = json.RawMessage(payloadJSON)

	c.JSON(http.StatusOK, p)
}

func (h *handler) createPage(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !auth.IsOwner(user, nil) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	var input ShrinePage
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	aboutJSON := string(input.About)
	keywordsJSON := string(input.Keywords)
	payloadJSON := string(input.Payload)

	_, err := h.db.Exec(`
		INSERT INTO shrine_pages (slug, path, title, description, excerpt, image, imageAlt,
			schemaType, aboutJson, keywordsJson, ctaLabel, priority, changefreq,
			payloadJson, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, input.Slug, input.Path, input.Title, input.Description, input.Excerpt, input.Image, input.ImageAlt,
		input.SchemaType, aboutJSON, keywordsJSON, input.CTALabel, input.Priority, input.Changefreq, payloadJSON)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Shrine page already exists or invalid data"})
		return
	}

	c.JSON(http.StatusCreated, input)
}

func (h *handler) updatePage(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil || !auth.IsOwner(user, nil) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	slug := c.Param("slug")
	var input ShrinePage
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	_, err := h.db.Exec(`
		UPDATE shrine_pages SET
			path = ?, title = ?, description = ?, excerpt = ?, image = ?, imageAlt = ?,
			schemaType = ?, aboutJson = ?, keywordsJson = ?, ctaLabel = ?,
			priority = ?, changefreq = ?, payloadJson = ?, updatedAt = datetime('now')
		WHERE slug = ?
	`, input.Path, input.Title, input.Description, input.Excerpt, input.Image, input.ImageAlt,
		input.SchemaType, string(input.About), string(input.Keywords), input.CTALabel,
		input.Priority, input.Changefreq, string(input.Payload), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shrine page not found"})
		return
	}

	c.JSON(http.StatusOK, input)
}

func (h *handler) hubSEOPage(c *gin.Context) {
	if !seo.IsCrawler(c.GetHeader("User-Agent")) {
		c.Redirect(http.StatusTemporaryRedirect, strings.TrimRight(h.cfg.FrontendURL, "/")+"/shrine")
		return
	}
	version := seo.VersionHash("shrine-hub-render-v1")
	seo.RenderShareHTML(c, h.cfg.WebsiteBase, seo.SharePage{
		Title:       "Character Shrines",
		Description: "Character shrine pages on Mirabellier.",
		Path:        "/shrine",
		ImagePath:   "/shrine/embed-image.png?v=" + version,
		ImageWidth:  embed.PreviewWidth,
		ImageHeight: embed.PreviewHeight,
		ImageAlt:    "A preview image of Mirabellier character shrines.",
	})
}

func (h *handler) shrineSEOPage(c *gin.Context) {
	page := h.loadShrinePage(c.Param("slug"))
	if page == nil {
		c.String(http.StatusNotFound, "Shrine page not found")
		return
	}
	if !seo.IsCrawler(c.GetHeader("User-Agent")) {
		c.Redirect(http.StatusTemporaryRedirect, strings.TrimRight(h.cfg.FrontendURL, "/")+"/shrine/"+page.Slug)
		return
	}
	desc := firstText(page.Description, page.Excerpt, "A character shrine on Mirabellier.")
	version := seo.VersionHash("shrine-render-v1", page.Slug, page.Title, desc, stringValue(page.Image), page.UpdatedAt)
	seo.RenderShareHTML(c, h.cfg.WebsiteBase, seo.SharePage{
		Title:       page.Title,
		Description: desc,
		Path:        "/shrine/" + page.Slug,
		ImagePath:   "/shrine/" + page.Slug + "/embed-image.png?v=" + version,
		ImageWidth:  embed.PreviewWidth,
		ImageHeight: embed.PreviewHeight,
		ImageAlt:    firstText(page.ImageAlt, nil, "A preview image of "+page.Title+"."),
	})
}

func (h *handler) hubEmbedImage(c *gin.Context) {
	png, err := embed.RenderShrineEmbed(embed.ShrinePreview{
		Title:       "Character Shrines",
		Description: "Character shrine pages on Mirabellier.",
		Slug:        "shrine",
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to render shrine preview image")
		return
	}
	c.Header("Content-Type", "image/png")
	c.Header("Content-Length", strconv.Itoa(len(png)))
	seo.SetEmbedImageCacheHeaders(c)
	c.Data(http.StatusOK, "image/png", png)
}

func (h *handler) shrineEmbedImage(c *gin.Context) {
	page := h.loadShrinePage(c.Param("slug"))
	if page == nil {
		c.String(http.StatusNotFound, "Shrine page not found")
		return
	}
	png, err := embed.RenderShrineEmbed(h.shrinePreviewFromPage(page))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to render shrine preview image")
		return
	}
	c.Header("Content-Type", "image/png")
	c.Header("Content-Length", strconv.Itoa(len(png)))
	seo.SetEmbedImageCacheHeaders(c)
	c.Data(http.StatusOK, "image/png", png)
}

func (h *handler) loadShrinePage(slug string) *ShrinePage {
	if page, ok := hardcodedShrines[slug]; ok {
		return &page
	}

	var p ShrinePage
	var aboutJSON, keywordsJSON, payloadJSON string
	err := h.db.QueryRow(`
		SELECT slug, path, title, description, excerpt, image, imageAlt, schemaType,
		       aboutJson, keywordsJson, ctaLabel, priority, changefreq, payloadJson,
		       createdAt, updatedAt
		FROM shrine_pages WHERE slug = ?
	`, slug).Scan(&p.Slug, &p.Path, &p.Title, &p.Description, &p.Excerpt, &p.Image, &p.ImageAlt,
		&p.SchemaType, &aboutJSON, &keywordsJSON, &p.CTALabel, &p.Priority, &p.Changefreq,
		&payloadJSON, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil
	}
	p.About = json.RawMessage(aboutJSON)
	p.Keywords = json.RawMessage(keywordsJSON)
	p.Payload = json.RawMessage(payloadJSON)
	return &p
}

func (h *handler) shrinePreviewFromPage(page *ShrinePage) embed.ShrinePreview {
	return embed.ShrinePreview{
		Title:       page.Title,
		Description: firstText(page.Description, page.Excerpt, "A character shrine on Mirabellier."),
		Image:       seo.PublicURL(h.cfg.WebsiteBase, stringValue(page.Image)),
		ImageAlt:    stringValue(page.ImageAlt),
		Slug:        page.Slug,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstText(primary, secondary *string, fallback string) string {
	if primary != nil && strings.TrimSpace(*primary) != "" {
		return *primary
	}
	if secondary != nil && strings.TrimSpace(*secondary) != "" {
		return *secondary
	}
	return fallback
}
