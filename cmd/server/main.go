// (◕‿◕) ✨ mirabellier ~ a cozy little corner of the internet!
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mirabellier/mirabellier-backend-go/internal/anime"
	"github.com/mirabellier/mirabellier-backend-go/internal/arena"
	"github.com/mirabellier/mirabellier-backend-go/internal/auth"
	"github.com/mirabellier/mirabellier-backend-go/internal/config"
	"github.com/mirabellier/mirabellier-backend-go/internal/database"
	"github.com/mirabellier/mirabellier-backend-go/internal/guestbook"
	"github.com/mirabellier/mirabellier-backend-go/internal/images"
	"github.com/mirabellier/mirabellier-backend-go/internal/middleware"
	"github.com/mirabellier/mirabellier-backend-go/internal/posts"
	"github.com/mirabellier/mirabellier-backend-go/internal/qotd"
	"github.com/mirabellier/mirabellier-backend-go/internal/quotes"
	"github.com/mirabellier/mirabellier-backend-go/internal/seo"
	"github.com/mirabellier/mirabellier-backend-go/internal/shrines"
)

func main() {
	godotenv.Load()
	cfg := config.Load()

	db, err := database.Open(cfg.DBFile)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	sitemap := seo.NewSitemap(db, cfg)
	if err := sitemap.Generate(); err != nil {
		log.Printf("Warning: sitemap generation failed: %v", err)
	}

	quotes.StartQuoteScheduler(db, cfg)
	qotd.StartDiscordScheduler(db, &qotd.DiscordConfig{
		QOTDDiscordWebhookURL:       cfg.QOTDDiscordWebhookURL,
		QOTDDiscordWebhookUsername:  cfg.QOTDDiscordWebhookUsername,
		QOTDDiscordWebhookAvatarURL: cfg.QOTDDiscordWebhookAvatarURL,
		WebsiteBase:                 cfg.WebsiteBase,
	})

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.KeepAlive())
	r.Use(middleware.CORS(cfg.AllowedOrigins()))

	seoPaths := []string{"/anime", "/blog/*id", "/profile/*username", "/question-of-the-day", "/quotes", "/shrine/*slug"}
	r.Use(middleware.VaryUserAgent(seoPaths...))

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(middleware.ServerTiming())
	r.Use(authMiddleware(db, cfg))

	auCfg := authConfig(cfg)
	postCfg := &posts.RouteConfig{DB: db, WebsiteBase: cfg.WebsiteBase}
	qo := qotdConfig(cfg)
	anCfg := animeConfig(cfg)

	// /v1 routes
	v1 := r.Group("/v1")
	{
		auth.RegisterRoutes(v1, db, auCfg)
		posts.RegisterRoutes(v1, postCfg)
		guestbook.RegisterRoutes(v1, db)
		qotd.RegisterRoutes(v1, db, qo)
		quotes.RegisterRoutes(v1, db, cfg)
		anime.RegisterRoutes(v1, db, anCfg)
		shrines.RegisterRoutes(v1, db)
		arena.RegisterRoutes(v1, db)
		images.RegisterRoutes(v1, db, cfg.ImagesDir)
	}

	// Routes without /v1 prefix
	noPrefix := r.Group("/")
	{
		auth.RegisterRoutes(noPrefix, db, auCfg)
		posts.RegisterRoutes(noPrefix, postCfg)
		guestbook.RegisterRoutes(noPrefix, db)
		qotd.RegisterRoutes(noPrefix, db, qo)
		quotes.RegisterRoutes(noPrefix, db, cfg)
		anime.RegisterRoutes(noPrefix, db, anCfg)
		shrines.RegisterRoutes(noPrefix, db)
		arena.RegisterRoutes(noPrefix, db)
		images.RegisterRoutes(noPrefix, db, cfg.ImagesDir)
	}

	r.NoRoute(images.ServeImageNoRoute(cfg.ImagesDir))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down...")
		db.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func authMiddleware(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return auth.Middleware(db, authConfig(cfg))
}

func authConfig(cfg *config.Config) *auth.Config {
	return &auth.Config{
		SessionSecret:              cfg.SessionSecret,
		SessionCookieName:          cfg.SessionCookieName,
		SessionCookieMaxAgeSeconds: cfg.SessionCookieMaxAgeSeconds,
		SessionCookieSecure:        cfg.SessionCookieSecure,
		OwnerDiscordIDs:            cfg.OwnerDiscordIDs,
		DiscordClientID:            cfg.DiscordClientID,
		DiscordClientSecret:        cfg.DiscordClientSecret,
		DiscordCallbackURL:         cfg.DiscordCallbackURL,
		FrontendURL:                cfg.FrontendURL,
		FrontendURLs:               cfg.FrontendURLs,
	}
}

func qotdConfig(cfg *config.Config) *qotd.Config {
	return &qotd.Config{
		QOTDDiscordWebhookURL:       cfg.QOTDDiscordWebhookURL,
		QOTDDiscordWebhookUsername:  cfg.QOTDDiscordWebhookUsername,
		QOTDDiscordWebhookAvatarURL: cfg.QOTDDiscordWebhookAvatarURL,
		OwnerDiscordIDs:             cfg.OwnerDiscordIDs,
		FrontendURL:                 cfg.FrontendURL,
		WebsiteBase:                 cfg.WebsiteBase,
	}
}

func animeConfig(cfg *config.Config) *anime.Config {
	return &anime.Config{
		MALClientID:            cfg.MALClientID,
		MALUsername:            cfg.MALUsername,
		MALAnimeRefreshMinutes: cfg.MALAnimeRefreshMinutes,
		FrontendURL:            cfg.FrontendURL,
		WebsiteBase:            cfg.WebsiteBase,
	}
}
