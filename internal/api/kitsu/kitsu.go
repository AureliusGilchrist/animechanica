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

// AnimeAssets contains non-name assets that can be used to enrich AniList data.
type AnimeAssets struct {
    PosterURL string `json:"posterUrl"`
    BannerURL string `json:"bannerUrl"`
    Synopsis  string `json:"synopsis"`
}

// animeSearchResponse models the subset of Kitsu anime response used for assets.
// Note: coverImage may not always be present; fields are optional and safely ignored if missing.
type animeSearchResponse struct {
    Data []struct {
        ID         string `json:"id"`
        Type       string `json:"type"`
        Attributes struct {
            Canonical   string `json:"canonicalTitle"`
            Synopsis    string `json:"synopsis"`
            PosterImage struct {
                Tiny     string `json:"tiny"`
                Small    string `json:"small"`
                Medium   string `json:"medium"`
                Large    string `json:"large"`
                Original string `json:"original"`
            } `json:"posterImage"`
            CoverImage struct {
                Tiny     string `json:"tiny"`
                Small    string `json:"small"`
                Large    string `json:"large"`
                Original string `json:"original"`
            } `json:"coverImage"`
        } `json:"attributes"`
    } `json:"data"`
}

// SearchAnimeAssetsByTitle queries Kitsu anime by title and returns poster, banner (if available) and synopsis.
// It prefers Original > Large > Medium > Small > Tiny for images.
func (c *Client) SearchAnimeAssetsByTitle(title string) (*AnimeAssets, error) {
    if title == "" {
        return &AnimeAssets{}, nil
    }
    u := "https://kitsu.io/api/edge/anime?" + url.Values{"filter[text]": {title}, "page[limit]": {"1"}}.Encode()
    req, _ := http.NewRequest(http.MethodGet, u, nil)
    req.Header.Set("Accept", "application/vnd.api+json")

    resp, err := c.HTTP.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("kitsu: http %d", resp.StatusCode)
    }

    var ar animeSearchResponse
    if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
        return nil, err
    }
    if len(ar.Data) == 0 {
        return &AnimeAssets{}, nil
    }

    attr := ar.Data[0].Attributes
    assets := &AnimeAssets{Synopsis: attr.Synopsis}

    // Poster selection
    if v := attr.PosterImage.Original; v != "" {
        assets.PosterURL = v
    } else if v := attr.PosterImage.Large; v != "" {
        assets.PosterURL = v
    } else if v := attr.PosterImage.Medium; v != "" {
        assets.PosterURL = v
    } else if v := attr.PosterImage.Small; v != "" {
        assets.PosterURL = v
    } else {
        assets.PosterURL = attr.PosterImage.Tiny
    }

    // Banner (cover image) selection if present
    if v := attr.CoverImage.Original; v != "" {
        assets.BannerURL = v
    } else if v := attr.CoverImage.Large; v != "" {
        assets.BannerURL = v
    } else if v := attr.CoverImage.Small; v != "" {
        assets.BannerURL = v
    } else if v := attr.CoverImage.Tiny; v != "" {
        assets.BannerURL = v
    }

    return assets, nil
}
