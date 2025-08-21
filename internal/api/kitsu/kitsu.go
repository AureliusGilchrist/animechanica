package kitsu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is a minimal Kitsu API client for manga covers.
type Client struct {
	HTTP *http.Client
}

type searchResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Slug        string `json:"slug"`
			Canonical   string `json:"canonicalTitle"`
			PosterImage struct {
				Tiny     string `json:"tiny"`
				Small    string `json:"small"`
				Medium   string `json:"medium"`
				Large    string `json:"large"`
				Original string `json:"original"`
			} `json:"posterImage"`
		} `json:"attributes"`
	} `json:"data"`
}

// New creates a new Kitsu client with a sensible timeout.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 8 * time.Second}}
}

// SearchPosterByTitle finds the best manga match by title and returns a poster image URL.
// It prefers Original > Large > Medium > Small > Tiny.
func (c *Client) SearchPosterByTitle(title string) (string, error) {
	if title == "" {
		return "", nil
	}
	u := "https://kitsu.io/api/edge/manga?" + url.Values{"filter[text]": {title}, "page[limit]": {"1"}}.Encode()
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("kitsu: http %d", resp.StatusCode)
	}
	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", err
	}
	if len(sr.Data) == 0 {
		return "", nil
	}
	pi := sr.Data[0].Attributes.PosterImage
	if pi.Original != "" {
		return pi.Original, nil
	}
	if pi.Large != "" {
		return pi.Large, nil
	}
	if pi.Medium != "" {
		return pi.Medium, nil
	}
	if pi.Small != "" {
		return pi.Small, nil
	}
	return pi.Tiny, nil
}
