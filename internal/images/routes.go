package images

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, db *sql.DB, imagesDir string) {
	h := &handler{db: db, imagesDir: imagesDir}

	r.GET("/images/list", h.list)
	r.GET("/images/meta/:filename", h.meta)
	r.POST("/posts-img", h.uploadPostImage)
}

// ServeImageNoRoute handles /images/* file serving as a NoRoute handler on gin.Engine.
func ServeImageNoRoute(imagesDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/images/") {
			c.Next()
			return
		}
		if path == "/images/list" || strings.HasPrefix(path, "/images/meta/") {
			c.Next()
			return
		}

		file := filepath.Base(path)
		if strings.Contains(file, "..") || strings.Contains(file, "\\") {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		fullPath := filepath.Join(imagesDir, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.File(fullPath)
		c.Abort()
	}
}

type handler struct {
	db        *sql.DB
	imagesDir string
}

func (h *handler) list(c *gin.Context) {
	entries, err := os.ReadDir(h.imagesDir)
	if err != nil {
		c.JSON(http.StatusOK, []any{})
		return
	}

	type ImageMeta struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		ModTime string `json:"modTime"`
	}

	var imgs []ImageMeta
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp.png") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !isImageFile(name) {
			continue
		}
		imgs = append(imgs, ImageMeta{
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	if imgs == nil {
		imgs = []ImageMeta{}
	}

	c.JSON(http.StatusOK, imgs)
}

func (h *handler) meta(c *gin.Context) {
	filename := filepath.Base(c.Param("filename"))
	if strings.Contains(filename, "..") || strings.Contains(filename, "\\") || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
		return
	}

	fullPath := filepath.Join(h.imagesDir, filename)
	info, err := os.Stat(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    filename,
		"size":    info.Size(),
		"modTime": info.ModTime().UTC().Format(time.RFC3339),
	})
}

func (h *handler) uploadPostImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image file provided"})
		return
	}

	if !isImageFile(file.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image format"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer src.Close()

	data := make([]byte, file.Size)
	if _, err := src.Read(data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
		return
	}

	result, err := SaveAndOptimize(data, file.Filename, h.imagesDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process image"})
		return
	}

	resp := gin.H{
		"image": fmt.Sprintf("/images/%s", result.JPEGName),
	}
	if result.WebPName != "" {
		resp["webp"] = fmt.Sprintf("/images/%s", result.WebPName)
	}

	c.JSON(http.StatusCreated, resp)
}

func isImageFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return true
	}
	return false
}
