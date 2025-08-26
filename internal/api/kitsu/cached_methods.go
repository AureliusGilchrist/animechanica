package kitsu

import (
    "context"
    "strings"
)

// CachedAnimeAssetsByTitle returns Kitsu anime assets by title using long TTL caching.
func CachedAnimeAssetsByTitle(ctx context.Context, title string, client *Client) (*AnimeAssets, error) {
    if client == nil {
        client = New()
    }
    // Normalize key (case-insensitive)
    norm := strings.TrimSpace(strings.ToLower(title))
    key := formatKey("anime_assets_by_title", norm)

    return CachedFetch[*AnimeAssets](bucketLong, key, func() (*AnimeAssets, error) {
        // ignore ctx for now since underlying client doesn't take it yet
        return client.SearchAnimeAssetsByTitle(title)
    })
}
