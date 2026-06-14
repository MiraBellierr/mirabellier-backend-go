package anime

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Config struct {
	MALClientID            string
	MALUsername            string
	MALAnimeRefreshMinutes int
	FrontendURL            string
}

type AnimeFeedPayload struct {
	Source    string               `json:"source"`
	Username  string               `json:"username"`
	FetchedAt string               `json:"fetchedAt"`
	Stale     bool                 `json:"stale"`
	Items     []AnimeFeedItem      `json:"items"`
}

type AnimeFeedItem struct {
	MalID            int         `json:"malId"`
	Title            string      `json:"title"`
	URL              string      `json:"url"`
	CoverImage       *string     `json:"coverImage"`
	MediaType        *string     `json:"mediaType"`
	WatchedEpisodes  int         `json:"watchedEpisodes"`
	TotalEpisodes    *int        `json:"totalEpisodes"`
	Score            *int        `json:"score"`
	UpdatedAt        *string     `json:"updatedAt"`
	StartSeason      *SeasonInfo `json:"startSeason"`
}

type SeasonInfo struct {
	Season string `json:"season"`
	Year   int    `json:"year"`
}

const ErrMALConfigMissing = "MAL_CONFIG_MISSING"
const ErrMALUnavailable = "MAL_UNAVAILABLE"

var inflightMAL sync.Map

func RegisterRoutes(r *gin.RouterGroup, db *sql.DB, cfg *Config) {
	h := &handler{db: db, cfg: cfg}
	r.GET("/anime", h.seoPage)
	r.GET("/anime/currently-watching/embed-image.png", h.embedImage)
	r.GET("/anime/currently-watching", h.getCurrentlyWatching)
}

type handler struct {
	db  *sql.DB
	cfg *Config
}

func (h *handler) seoPage(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, h.cfg.FrontendURL+"/anime")
}

func (h *handler) embedImage(c *gin.Context) {
	c.JSON(501, gin.H{"error": "not implemented"})
}

func (h *handler) getCurrentlyWatching(c *gin.Context) {
	if h.cfg.MALClientID == "" || h.cfg.MALUsername == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MAL configuration missing",
			"code":  ErrMALConfigMissing,
		})
		return
	}

	feedKey := "currently-watching"

	// Deduplicate concurrent requests
	if _, loading := inflightMAL.LoadOrStore(feedKey, true); loading {
		if payload := getCachedPayload(h.db, feedKey); payload != nil {
			payload.Stale = true
			c.JSON(http.StatusOK, payload)
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "MAL currently unavailable",
			"code":  ErrMALUnavailable,
		})
		return
	}
	defer inflightMAL.Delete(feedKey)

	// Return fresh cache
	if isCacheFresh(h.db, feedKey, h.cfg.MALAnimeRefreshMinutes) {
		if payload := getCachedPayload(h.db, feedKey); payload != nil {
			c.JSON(http.StatusOK, payload)
			return
		}
	}

	// Fetch from MAL
	payload, err := fetchFromMAL(h.cfg)
	if err != nil {
		if snapshot := getCachedPayload(h.db, feedKey); snapshot != nil {
			snapshot.Stale = true
			c.JSON(http.StatusOK, snapshot)
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "MAL currently unavailable",
			"code":  ErrMALUnavailable,
		})
		return
	}

	saveSnapshot(h.db, feedKey, payload)
	c.JSON(http.StatusOK, payload)
}

func fetchFromMAL(cfg *Config) (*AnimeFeedPayload, error) {
	url := fmt.Sprintf("https://api.myanimelist.net/v2/users/%s/animelist", cfg.MALUsername)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mirabellier/1.0 (+https://mirabellier.com)")
	req.Header.Set("X-MAL-CLIENT-ID", cfg.MALClientID)

	q := req.URL.Query()
	q.Set("status", "watching")
	q.Set("sort", "list_updated_at")
	q.Set("limit", "1000")
	q.Set("fields", "list_status,num_episodes,main_picture,media_type,start_season,alternative_titles")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MAL API returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []struct {
			Node struct {
				ID                int    `json:"id"`
				Title             string `json:"title"`
				NumEpisodes       int    `json:"num_episodes"`
				MediaType         string `json:"media_type"`
				MainPicture       *struct {
					Large  string `json:"large"`
					Medium string `json:"medium"`
				} `json:"main_picture"`
				AlternativeTitles *struct {
					EN string `json:"en"`
				} `json:"alternative_titles"`
				StartSeason *struct {
					Season string `json:"season"`
					Year   int    `json:"year"`
				} `json:"start_season"`
			} `json:"node"`
			ListStatus *struct {
				NumEpisodesWatched int    `json:"num_episodes_watched"`
				Score              int    `json:"score"`
				UpdatedAt          string `json:"updated_at"`
			} `json:"list_status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var items []AnimeFeedItem
	for _, entry := range raw.Data {
		node := entry.Node

		malID := node.ID
		if malID <= 0 {
			continue
		}

		title := node.Title
		if node.AlternativeTitles != nil && node.AlternativeTitles.EN != "" {
			title = node.AlternativeTitles.EN
		}

		var coverImage *string
		if node.MainPicture != nil {
			if node.MainPicture.Large != "" {
				coverImage = &node.MainPicture.Large
			} else if node.MainPicture.Medium != "" {
				coverImage = &node.MainPicture.Medium
			}
		}

		var mediaType *string
		if node.MediaType != "" {
			mediaType = &node.MediaType
		}

		var totalEpisodes *int
		if node.NumEpisodes > 0 {
			totalEpisodes = &node.NumEpisodes
		}

		var score *int
		var watchedEpisodes int
		var updatedAt *string
		if entry.ListStatus != nil {
			if entry.ListStatus.Score > 0 {
				score = &entry.ListStatus.Score
			}
			watchedEpisodes = max(0, entry.ListStatus.NumEpisodesWatched)
			if entry.ListStatus.UpdatedAt != "" {
				updatedAt = &entry.ListStatus.UpdatedAt
			}
		}

		var startSeason *SeasonInfo
		if node.StartSeason != nil && node.StartSeason.Season != "" && node.StartSeason.Year > 0 {
			startSeason = &SeasonInfo{
				Season: node.StartSeason.Season,
				Year:   node.StartSeason.Year,
			}
		}

		items = append(items, AnimeFeedItem{
			MalID:           malID,
			Title:           title,
			URL:             fmt.Sprintf("https://myanimelist.net/anime/%d", malID),
			CoverImage:      coverImage,
			MediaType:       mediaType,
			WatchedEpisodes: watchedEpisodes,
			TotalEpisodes:   totalEpisodes,
			Score:           score,
			UpdatedAt:       updatedAt,
			StartSeason:     startSeason,
		})
	}

	// Sort by updatedAt descending (most recently updated first)
	sort.Slice(items, func(i, j int) bool {
		ti := time.Time{}
		tj := time.Time{}
		if items[i].UpdatedAt != nil {
			ti, _ = time.Parse(time.RFC3339, *items[i].UpdatedAt)
		}
		if items[j].UpdatedAt != nil {
			tj, _ = time.Parse(time.RFC3339, *items[j].UpdatedAt)
		}
		return ti.After(tj)
	})

	return &AnimeFeedPayload{
		Source:    "myanimelist",
		Username:  cfg.MALUsername,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Stale:     false,
		Items:     items,
	}, nil
}

func isCacheFresh(db *sql.DB, feedKey string, ttlMinutes int) bool {
	var fetchedAt string
	err := db.QueryRow(`
		SELECT fetchedAt FROM myanimelist_anime_snapshots
		WHERE feedKey = ? ORDER BY fetchedAt DESC LIMIT 1
	`, feedKey).Scan(&fetchedAt)
	if err != nil {
		return false
	}

	t, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return false
	}

	return time.Since(t) < time.Duration(ttlMinutes)*time.Minute
}

func getCachedPayload(db *sql.DB, feedKey string) *AnimeFeedPayload {
	row := db.QueryRow(`
		SELECT username, fetchedAt, payloadJson
		FROM myanimelist_anime_snapshots
		WHERE feedKey = ? ORDER BY fetchedAt DESC LIMIT 1
	`, feedKey)

	var username, fetchedAt, payloadJSON string
	if err := row.Scan(&username, &fetchedAt, &payloadJSON); err != nil {
		return nil
	}

	var items []AnimeFeedItem
	if err := json.Unmarshal([]byte(payloadJSON), &items); err != nil {
		return nil
	}

	return &AnimeFeedPayload{
		Source:    "myanimelist",
		Username:  username,
		FetchedAt: fetchedAt,
		Stale:     false,
		Items:     items,
	}
}

func saveSnapshot(db *sql.DB, feedKey string, payload *AnimeFeedPayload) {
	itemsJSON, _ := json.Marshal(payload.Items)
	now := time.Now().UTC().Format(time.RFC3339)

	db.Exec(`
		INSERT INTO myanimelist_anime_snapshots (feedKey, username, fetchedAt, payloadJson, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(feedKey) DO UPDATE SET
			username = excluded.username,
			fetchedAt = excluded.fetchedAt,
			payloadJson = excluded.payloadJson,
			updatedAt = excluded.updatedAt
	`, feedKey, payload.Username, now, string(itemsJSON), now, now)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
