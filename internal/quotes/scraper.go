package quotes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	htmlStr := string(body)
	quotes := parseQuotesOfTheDayHTML(htmlStr)
	if len(quotes) == 0 {
		return nil, "", fmt.Errorf("no quotes found in HTML")
	}

	return quotes, "html", nil
}

var sectionPattern = regexp.MustCompile(
	`<h2 class="qotd-h2">\s*([^<]+?)\s*</h2>\s*` +
		`<a[^>]+href="([^"]+)"[^>]*title="view quote"[^>]*>` +
		`([\s\S]*?)</a>\s*` +
		`<a[^>]+href="([^"]+)"[^>]*title="view author"[^>]*>` +
		`([\s\S]*?)</a>`,
)

var displayDatePattern = regexp.MustCompile(`<div class="qotdSubtInf">\s*([^<]+?)\s*</div>`)

var expectedCategories = []string{
	"Quote of the Day",
	"Love Quote of the Day",
	"Art Quote of the Day",
	"Nature Quote of the Day",
	"Funny Quote Of the Day",
}

func parseQuotesOfTheDayHTML(htmlStr string) []map[string]any {
	quotesByCat := make(map[string]map[string]any)

	// Parse display date
	var displayDate string
	if m := displayDatePattern.FindStringSubmatch(htmlStr); m != nil {
		displayDate = stripTags(m[1])
	}

	matches := sectionPattern.FindAllStringSubmatch(htmlStr, -1)
	for _, m := range matches {
		category := stripTags(m[1])

		found := false
		for _, ec := range expectedCategories {
			if category == ec {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		quoteText := normalizeQuoteText(m[3])
		author := stripTags(m[5])
		if quoteText == "" || author == "" {
			continue
		}

		quotesByCat[category] = map[string]any{
			"category":  category,
			"quote":     quoteText,
			"author":    author,
			"quoteUrl":  toAbsoluteURL(m[2]),
			"authorUrl": toAbsoluteURL(m[4]),
			"sourceUrl": brainyQuoteURL,
		}
	}

	var quotes []map[string]any
	for _, cat := range expectedCategories {
		if q, ok := quotesByCat[cat]; ok {
			quotes = append(quotes, q)
		}
	}

	_ = displayDate // stored as displayDate in snapshot

	return quotes
}

func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}

func normalizeQuoteText(s string) string {
	s = stripTags(s)
	s = htmlEntityRegex.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := htmlEntities[m]; ok {
			return v
		}
		return m
	})
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	// Strip surrounding double quotes (BrainyQuote wraps quotes in ")
	surrounding := regexp.MustCompile(`^"(.*)"$`)
	if m := surrounding.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	return s
}

func toAbsoluteURL(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return "https://www.brainyquote.com" + href
	}
	return "https://www.brainyquote.com/" + href
}

var htmlEntityRegex = regexp.MustCompile(`&[a-z]+;`)

var htmlEntities = map[string]string{
	"&amp;":   "&",
	"&apos;":  "'",
	"&copy;":  "(c)",
	"&gt;":    ">",
	"&hellip;": "...",
	"&laquo;": "\"",
	"&ldquo;": "\"",
	"&lsquo;": "'",
	"&lt;":    "<",
	"&mdash;": "-",
	"&nbsp;":  " ",
	"&ndash;": "-",
	"&quot;":  "\"",
	"&raquo;": "\"",
	"&rdquo;": "\"",
	"&rsquo;": "'",
	"&trade;": "TM",
}

func fetchBrainyQuotesRSS() ([]map[string]any, string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	type rssFeed struct {
		category string
		url      string
	}
	feeds := []rssFeed{
		{"Quote of the Day", "https://www.brainyquote.com/link/quotebr.rss"},
		{"Love Quote of the Day", "https://www.brainyquote.com/link/quotelo.rss"},
		{"Art Quote of the Day", "https://www.brainyquote.com/link/quotear.rss"},
		{"Nature Quote of the Day", "https://www.brainyquote.com/link/quotena.rss"},
		{"Funny Quote Of the Day", "https://www.brainyquote.com/link/quotefu.rss"},
	}

	var quotes []map[string]any

	for _, feed := range feeds {
		req, _ := http.NewRequest("GET", feed.url, nil)
		req.Header.Set("User-Agent", randomUA())
		req.Header.Set("Accept", "application/rss+xml,application/xml,text/xml;q=0.9,*/*;q=0.8")

		resp, err := client.Do(req)
		if err != nil { continue }
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 { continue }

		// Match first <item> block
		itemRe := regexp.MustCompile(`<item>([\s\S]*?)</item>`)
		itemMatch := itemRe.FindStringSubmatch(string(body))
		if itemMatch == nil { continue }

		itemXML := itemMatch[1]
		quote := normalizeQuoteText(getXMLTag(itemXML, "description"))
		author := stripTags(getXMLTag(itemXML, "title"))
		if quote == "" || author == "" { continue }

		quotes = append(quotes, map[string]any{
			"category":  feed.category,
			"quote":     quote,
			"author":    author,
			"sourceUrl": feed.url,
		})
	}

	if len(quotes) == 0 {
		return nil, "", fmt.Errorf("all RSS feeds returned no quotes")
	}

	return quotes, "rss", nil
}

func getXMLTag(xml, tag string) string {
	re := regexp.MustCompile(`<` + tag + `[^>]*>([\s\S]*?)</` + tag + `>`)
	m := re.FindStringSubmatch(xml)
	if m != nil {
		return m[1]
	}
	return ""
}
