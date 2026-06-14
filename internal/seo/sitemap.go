package seo

import (
	"database/sql"
	"encoding/xml"
	"os"
	"path/filepath"

	"github.com/mirabellier/mirabellier-backend-go/internal/config"
)

type urlEntry struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	LastMod    string   `xml:"lastmod,omitempty"`
	Changefreq string   `xml:"changefreq,omitempty"`
	Priority   string   `xml:"priority,omitempty"`
}

type Sitemap struct {
	db  *sql.DB
	cfg *config.Config
}

func NewSitemap(db *sql.DB, cfg *config.Config) *Sitemap {
	return &Sitemap{db: db, cfg: cfg}
}

func (s *Sitemap) Generate() error {
	entries := s.collectEntries()

	type sitemapXML struct {
		XMLName xml.Name   `xml:"urlset"`
		Xmlns   string     `xml:"xmlns,attr"`
		URLs    []urlEntry `xml:"url"`
	}

	sm := sitemapXML{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}

	output, err := xml.MarshalIndent(sm, "", "  ")
	if err != nil {
		return err
	}

	xmlContent := xml.Header + string(output)

	var outPath string
	if s.cfg.FrontendDeployPath != "" {
		outPath = filepath.Join(s.cfg.FrontendDeployPath, "sitemap.xml")
	} else {
		outPath = filepath.Join("..", "public", "sitemap.xml")
	}

	return os.WriteFile(outPath, []byte(xmlContent), 0644)
}

func (s *Sitemap) collectEntries() []urlEntry {
	base := s.cfg.WebsiteBase
	entries := []urlEntry{
		{Loc: base + "/", Priority: "1.0"},
		{Loc: base + "/home", Priority: "0.8"},
		{Loc: base + "/about", Priority: "0.7"},
		{Loc: base + "/projects", Priority: "0.7"},
		{Loc: base + "/anime", Priority: "0.8"},
		{Loc: base + "/question-of-the-day", Priority: "0.8"},
		{Loc: base + "/question-of-the-day/archive", Priority: "0.6"},
		{Loc: base + "/quotes", Priority: "0.8"},
		{Loc: base + "/blog", Priority: "0.9"},
	}

	rows, err := s.db.Query("SELECT path FROM shrine_pages")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var path string
			rows.Scan(&path)
			entries = append(entries, urlEntry{Loc: base + path, Priority: "0.7"})
		}
	}

	rows2, err := s.db.Query("SELECT id, updatedAt FROM posts ORDER BY createdAt DESC")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id, updatedAt string
			rows2.Scan(&id, &updatedAt)
			entries = append(entries, urlEntry{Loc: base + "/blog/" + id, LastMod: updatedAt})
		}
	}

	rows3, err := s.db.Query("SELECT recordedDate FROM daily_questions WHERE archivedAt IS NOT NULL ORDER BY recordedDate DESC")
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var date string
			rows3.Scan(&date)
			entries = append(entries, urlEntry{Loc: base + "/question-of-the-day/archive/" + date})
		}
	}

	return entries
}
