package arena

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

const JIKAN_API_BASE = "https://api.jikan.moe/v4"
const SOURCE_TIMEOUT_MS = 15000
const MAX_SOURCE_RETRIES = 3
const CHARACTER_ID_MIN = 1
const CHARACTER_ID_MAX = 44000
const MAX_RANDOM_ID_ATTEMPTS = 10

type MALCharacterCard struct {
	MalID      int     `json:"malId"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	ImageURL   string  `json:"imageUrl"`
	MeanScore  *float64 `json:"meanScore"`
	Popularity *int    `json:"popularity"`
	Favorites  *int    `json:"favorites"`
	NSFW       string  `json:"nsfw"`
}

func DrawArenaCard(db *sql.DB) (*MALCharacterCard, error) {
	for attempt := 0; attempt < MAX_RANDOM_ID_ATTEMPTS; attempt++ {
		charID := rand.Intn(CHARACTER_ID_MAX-CHARACTER_ID_MIN+1) + CHARACTER_ID_MIN
		card, err := fetchJikanCharacter(charID)
		if err != nil {
			continue
		}
		return card, nil
	}

	card, err := pickFromDBPool(db)
	if err != nil {
		return nil, fmt.Errorf("arena card source unavailable: %w", err)
	}
	return card, nil
}

func fetchJikanCharacter(charID int) (*MALCharacterCard, error) {
	url := fmt.Sprintf("%s/characters/%d", JIKAN_API_BASE, charID)

	var lastErr error
	for attempt := 0; attempt < MAX_SOURCE_RETRIES; attempt++ {
		client := &http.Client{Timeout: SOURCE_TIMEOUT_MS * time.Millisecond}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mirabellier Arena/1.0 (+https://mirabellier.com)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < MAX_SOURCE_RETRIES-1 {
				time.Sleep(time.Duration(attempt+1) * 1200 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("jikan rate limited (status %d)", resp.StatusCode)
			if attempt < MAX_SOURCE_RETRIES-1 {
				time.Sleep(time.Duration(attempt+1) * 1200 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			return nil, fmt.Errorf("character not found")
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		var result struct {
			Data struct {
				MalID     int    `json:"mal_id"`
				Name      string `json:"name"`
				Favorites int   `json:"favorites"`
				Rank      int   `json:"rank"`
				Images    struct {
					JPG struct {
						LargeImageURL string `json:"large_image_url"`
						ImageURL      string `json:"image_url"`
					} `json:"jpg"`
					WebP struct {
						LargeImageURL string `json:"large_image_url"`
						ImageURL      string `json:"image_url"`
					} `json:"webp"`
				} `json:"images"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		imageURL := result.Data.Images.JPG.LargeImageURL
		if imageURL == "" {
			imageURL = result.Data.Images.WebP.LargeImageURL
		}
		if imageURL == "" {
			imageURL = result.Data.Images.JPG.ImageURL
		}
		if imageURL == "" {
			continue
		}

		card := &MALCharacterCard{
			MalID:    result.Data.MalID,
			Title:    result.Data.Name,
			URL:      fmt.Sprintf("https://myanimelist.net/character/%d", result.Data.MalID),
			ImageURL: imageURL,
			Favorites: &result.Data.Favorites,
			NSFW:     "unknown",
		}

		meanScore := inferMeanScore(result.Data.Favorites)
		card.MeanScore = &meanScore

		if result.Data.Rank > 0 {
			card.Popularity = &result.Data.Rank
		}

		return card, nil
	}

	return nil, lastErr
}

func inferMeanScore(favorites int) float64 {
	if favorites <= 0 {
		return 0
	}
	inferred := 6.0 + math.Log10(float64(favorites)+1)/1.25
	return math.Round(clampFloat(inferred, 6, 10)*100) / 100
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func RarityFromFavorites(favorites *int) string {
	if favorites == nil || *favorites <= 0 {
		return "C"
	}
	clamped := *favorites
	if clamped < CHARACTER_FAVORITES_MIN {
		clamped = CHARACTER_FAVORITES_MIN
	}
	if clamped > CHARACTER_FAVORITES_MAX {
		clamped = CHARACTER_FAVORITES_MAX
	}

	low := math.Log(float64(CHARACTER_FAVORITES_MIN))
	high := math.Log(float64(CHARACTER_FAVORITES_MAX))
	progress := 0.0
	if high > low {
		progress = (math.Log(float64(clamped)) - low) / (high - low)
	}

	weightSum := 0
	for _, r := range RarityOrder {
		weightSum += RarityConfigMap[r].Weight
	}

	cursor := 0
	for _, r := range RarityOrder {
		cursor += RarityConfigMap[r].Weight
		if progress < float64(cursor)/float64(weightSum) {
			return r
		}
	}

	return "UR"
}

func pickFromDBPool(db *sql.DB) (*MALCharacterCard, error) {
	var malID int
	var title, url, imageURL string
	var meanScore *float64
	var popularity, favorites *int
	var nsfw string

	err := db.QueryRow(`
		SELECT malId, title, url, imageUrl, meanScore, popularity, favorites, nsfw
		FROM arena_mal_card_pool ORDER BY RANDOM() LIMIT 1
	`).Scan(&malID, &title, &url, &imageURL, &meanScore, &popularity, &favorites, &nsfw)
	if err != nil {
		return nil, err
	}

	return &MALCharacterCard{
		MalID: malID, Title: title, URL: url, ImageURL: imageURL,
		MeanScore: meanScore, Popularity: popularity, Favorites: favorites, NSFW: nsfw,
	}, nil
}
