package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                        int
	DBFile                      string
	SessionSecret               string
	OwnerDiscordIDs             []string
	SessionCookieName           string
	SessionCookieMaxAgeSeconds  int
	SessionCookieSecure         bool
	ImagesDir                   string
	DiscordClientID             string
	DiscordClientSecret         string
	DiscordCallbackURL          string
	FrontendURL                 string
	FrontendURLs                []string
	FrontendDeployPath          string
	SpaEntryFile                string
	MALClientID                 string
	MALUsername                 string
	MALAnimeRefreshMinutes      int
	QOTDDiscordWebhookURL       string
	QOTDDiscordWebhookUsername  string
	QOTDDiscordWebhookAvatarURL string
	QuoteFetchHourUTC           int
	QuoteFetchMinuteUTC         int
	WebsiteBase                 string
	IndexNowKey                 string
	IndexNowEnabled             bool
	IndexNowEndpoint            string
	LogAuthSignatureMismatch    bool
}

func Load() *Config {
	cfg := &Config{
		Port:                       getEnvInt("PORT", 5000),
		DBFile:                     getEnv("DB_FILE", "./database.sqlite3"),
		SessionSecret:              requireEnv("SESSION_SECRET"),
		OwnerDiscordIDs:            splitEnv("OWNER_DISCORD_IDS", "548050617889980426"),
		SessionCookieName:          getEnv("SESSION_COOKIE_NAME", "mirabellier_session"),
		SessionCookieMaxAgeSeconds: getEnvInt("SESSION_COOKIE_MAX_AGE_SECONDS", 2592000),
		ImagesDir:                  getEnv("IMAGES_DIR", "images"),
		DiscordClientID:            requireEnv("DISCORD_CLIENT_ID"),
		DiscordClientSecret:        requireEnv("DISCORD_CLIENT_SECRET"),
		DiscordCallbackURL:         getEnv("DISCORD_CALLBACK_URL", "http://localhost:3000/auth/discord/callback"),
		FrontendURL:                getEnv("FRONTEND_URL", "http://localhost:5173"),
		MALClientID:                requireEnv("MAL_CLIENT_ID"),
		MALUsername:                requireEnv("MAL_USERNAME"),
		MALAnimeRefreshMinutes:     getEnvInt("MAL_ANIME_REFRESH_MINUTES", 5),
		QOTDDiscordWebhookURL:      os.Getenv("QOTD_DISCORD_WEBHOOK_URL"),
		QOTDDiscordWebhookUsername: getEnv("QOTD_DISCORD_WEBHOOK_USERNAME", "Mirabellier QOTD"),
		QOTDDiscordWebhookAvatarURL: os.Getenv("QOTD_DISCORD_WEBHOOK_AVATAR_URL"),
		QuoteFetchHourUTC:          getEnvInt("QUOTE_FETCH_HOUR_UTC", 0),
		QuoteFetchMinuteUTC:        getEnvInt("QUOTE_FETCH_MINUTE_UTC", 0),
		WebsiteBase:                getEnv("WEBSITE_BASE", "https://mirabellier.com"),
		IndexNowKey:                os.Getenv("INDEXNOW_KEY"),
		IndexNowEndpoint:           getEnv("INDEXNOW_ENDPOINT", "https://api.indexnow.org/indexnow"),
		FrontendDeployPath:         os.Getenv("FRONTEND_DEPLOY_PATH"),
		SpaEntryFile:               os.Getenv("SPA_ENTRY_FILE"),
		LogAuthSignatureMismatch:   getEnvBool("LOG_AUTH_SIGNATURE_MISMATCH", false),
	}

	if val := os.Getenv("SESSION_COOKIE_SECURE"); val != "" {
		cfg.SessionCookieSecure, _ = strconv.ParseBool(val)
	}

	frontendURLs := os.Getenv("FRONTEND_URLS")
	if frontendURLs != "" {
		cfg.FrontendURLs = strings.Split(frontendURLs, ",")
	}

	if val := os.Getenv("INDEXNOW_ENABLED"); val != "" {
		cfg.IndexNowEnabled, _ = strconv.ParseBool(val)
	} else {
		cfg.IndexNowEnabled = cfg.IndexNowKey != ""
	}

	return cfg
}

func (c *Config) AllowedOrigins() []string {
	origins := []string{
		c.FrontendURL,
		"https://mirabellier.com",
		"https://www.mirabellier.com",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	}
	origins = append(origins, c.FrontendURLs...)
	return unique(origins)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic("required env var not set: " + key)
	}
	return val
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

func splitEnv(key, defaultVal string) []string {
	val := os.Getenv(key)
	if val == "" {
		val = defaultVal
	}
	return strings.Split(val, ",")
}

func unique(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
