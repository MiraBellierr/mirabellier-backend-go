package quotes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const brainyQuoteURL = "https://www.brainyquote.com/quote_of_the_day"

var rssFeedURLs = []string{
	"https://www.brainyquote.com/link/quotebr.rss",
	"https://www.brainyquote.com/link/quotelo.rss",
	"https://www.brainyquote.com/link/quotear.rss",
	"https://www.brainyquote.com/link/quotena.rss",
	"https://www.brainyquote.com/link/quotefu.rss",
}

var inflightRefresh sync.Map
var userAgents = []string{
	"Mozilla/5.0 Codex/1.0",
	"Mozilla/5.0 Codex/1.0 Accept-Language: en-US,en;q=0.9",
}

func randomUA() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// RefreshQuoteSnapshot fetches the daily quote and stores it.
func RefreshQuoteSnapshot(db *sql.DB, date time.Time) error {
	key := date.UTC().Format("2006-01-02")
	if _, loaded := inflightRefresh.LoadOrStore(key, true); loaded {
		return nil // already in progress
	}
	defer inflightRefresh.Delete(key)

	quotes, sourceType, err := fetchBrainyQuotes()
	if err != nil {
		quotes, sourceType, err = fetchBrainyQuotesRSS()
		if err != nil {
			return err
		}
	}

	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	quotesJSON, _ := json.Marshal(quotes)

	_, err = db.Exec(`
		INSERT INTO quote_snapshots (recordedDate, provider, sourceType, fetchedAt, quotesJson, createdAt, updatedAt)
		VALUES (?, 'BrainyQuote', ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(recordedDate) DO UPDATE SET
			fetchedAt = excluded.fetchedAt, quotesJson = excluded.quotesJson,
			sourceType = excluded.sourceType, updatedAt = datetime('now')
	`, key, sourceType, fetchedAt, string(quotesJSON))

	return err
}

func fetchBrainyQuotes() ([]map[string]any, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", brainyQuoteURL, nil)
	req.Header.Set("User-Agent", randomUA())
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("brainyquote returned status %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var quotes []map[string]any
	categories := []string{"general", "love", "art", "nature", "funny"}
	categoryIdx := 0

	extractFromHTML(doc, &quotes, &categoryIdx, categories)

	if len(quotes) == 0 {
		return nil, "", fmt.Errorf("no quotes found in HTML")
	}

	return quotes, "html", nil
}

func extractFromHTML(n *html.Node, quotes *[]map[string]any, categoryIdx *int, categories []string) {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "title" && strings.Contains(attr.Val, "view quote") {
				text := extractText(n)
				if text != "" && *categoryIdx < len(categories) {
					parts := strings.SplitN(text, "\n", 2)
					quote := strings.TrimSpace(parts[0])
					author := ""
					if len(parts) > 1 {
						author = strings.TrimSpace(parts[1])
					}
					if quote != "" {
						*quotes = append(*quotes, map[string]any{
							"quote":    quote,
							"author":   author,
							"category": categories[*categoryIdx],
						})
						*categoryIdx++
					}
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractFromHTML(c, quotes, categoryIdx, categories)
	}
}

func extractText(n *html.Node) string {
	var buf strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return buf.String()
}

func fetchBrainyQuotesRSS() ([]map[string]any, string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error

	for _, feedURL := range rssFeedURLs {
		req, _ := http.NewRequest("GET", feedURL, nil)
		req.Header.Set("User-Agent", randomUA())

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != 200 {
			continue
		}

		quotes := parseRSSXML(string(body))
		if len(quotes) > 0 {
			return quotes, "rss", nil
		}
	}

	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("all RSS feeds returned no quotes")
}

func parseRSSXML(xmlContent string) []map[string]any {
	doc, err := html.Parse(strings.NewReader(xmlContent))
	if err != nil {
		return nil
	}

	var quotes []map[string]any
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "item" {
			var title, description string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					switch c.Data {
					case "title":
						title = extractText(c)
					case "description":
						description = extractText(c)
					}
				}
			}
			if title != "" {
				quote := map[string]any{
					"quote":  title,
					"author": description,
				}
				quotes = append(quotes, quote)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return quotes
}
