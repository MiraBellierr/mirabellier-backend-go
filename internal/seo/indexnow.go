package seo

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/mirabellier/mirabellier-backend-go/internal/config"
)

func SubmitToIndexNow(urls []string, cfg *config.Config) error {
	if !cfg.IndexNowEnabled || cfg.IndexNowKey == "" {
		return nil
	}

	body := map[string]interface{}{
		"host":    cfg.WebsiteBase,
		"key":     cfg.IndexNowKey,
		"urlList": urls,
	}

	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(cfg.IndexNowEndpoint, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
