package anilist

import (
    "context"
)

// Cached wrappers for commonly used AniList calls.
// These use in-memory + file-backed cache with inflight request coalescing.

func (ac *AnilistClientImpl) CachedAnimeCollection(ctx context.Context, userName *string) (*AnimeCollection, error) {
    key := formatKey("anime_collection", valOr("me", userName))
    return CachedFetch[*AnimeCollection](bucketMedium, key, func() (*AnimeCollection, error) {
        return ac.AnimeCollection(ctx, userName)
    })
}

func (ac *AnilistClientImpl) CachedAnimeCollectionWithRelations(ctx context.Context, userName *string) (*AnimeCollectionWithRelations, error) {
    key := formatKey("anime_collection_rel", valOr("me", userName))
    return CachedFetch[*AnimeCollectionWithRelations](bucketMedium, key, func() (*AnimeCollectionWithRelations, error) {
        return ac.AnimeCollectionWithRelations(ctx, userName)
    })
}

func (ac *AnilistClientImpl) CachedMangaCollection(ctx context.Context, userName *string) (*MangaCollection, error) {
    key := formatKey("manga_collection", valOr("me", userName))
    return CachedFetch[*MangaCollection](bucketMedium, key, func() (*MangaCollection, error) {
        return ac.MangaCollection(ctx, userName)
    })
}

func (ac *AnilistClientImpl) CachedBaseAnimeByID(ctx context.Context, id *int) (*BaseAnimeByID, error) {
    key := formatKey("base_anime_by_id", valOr(0, id))
    return CachedFetch[*BaseAnimeByID](bucketLong, key, func() (*BaseAnimeByID, error) {
        return ac.BaseAnimeByID(ctx, id)
    })
}

func (ac *AnilistClientImpl) CachedAnimeDetailsByID(ctx context.Context, id *int) (*AnimeDetailsByID, error) {
    key := formatKey("anime_details_by_id", valOr(0, id))
    return CachedFetch[*AnimeDetailsByID](bucketLong, key, func() (*AnimeDetailsByID, error) {
        return ac.AnimeDetailsByID(ctx, id)
    })
}

func (ac *AnilistClientImpl) CachedBaseMangaByID(ctx context.Context, id *int) (*BaseMangaByID, error) {
    key := formatKey("base_manga_by_id", valOr(0, id))
    return CachedFetch[*BaseMangaByID](bucketLong, key, func() (*BaseMangaByID, error) {
        return ac.BaseMangaByID(ctx, id)
    })
}

func (ac *AnilistClientImpl) CachedMangaDetailsByID(ctx context.Context, id *int) (*MangaDetailsByID, error) {
    key := formatKey("manga_details_by_id", valOr(0, id))
    return CachedFetch[*MangaDetailsByID](bucketLong, key, func() (*MangaDetailsByID, error) {
        return ac.MangaDetailsByID(ctx, id)
    })
}

func (ac *AnilistClientImpl) CachedAnimeAiringSchedule(ctx context.Context, ids []*int, season *MediaSeason, seasonYear *int, previousSeason *MediaSeason, previousSeasonYear *int, nextSeason *MediaSeason, nextSeasonYear *int) (*AnimeAiringSchedule, error) {
    // Airing schedule changes frequently -> short TTL
    key := formatKey("airing_schedule", ids, season, valOr(0, seasonYear), previousSeason, valOr(0, previousSeasonYear), nextSeason, valOr(0, nextSeasonYear))
    return CachedFetch[*AnimeAiringSchedule](bucketShort, key, func() (*AnimeAiringSchedule, error) {
        return ac.AnimeAiringSchedule(ctx, ids, season, seasonYear, previousSeason, previousSeasonYear, nextSeason, nextSeasonYear)
    })
}

func (ac *AnilistClientImpl) CachedViewerStats(ctx context.Context) (*ViewerStats, error) {
    key := formatKey("viewer_stats")
    return CachedFetch[*ViewerStats](bucketMedium, key, func() (*ViewerStats, error) {
        return ac.ViewerStats(ctx)
    })
}

func (ac *AnilistClientImpl) CachedViewerFull(ctx context.Context) (*ViewerFull, error) {
    key := formatKey("viewer_full")
    return CachedFetch[*ViewerFull](bucketMedium, key, func() (*ViewerFull, error) {
        return ac.ViewerFull(ctx)
    })
}

// helper to deref pointer values safely
func valOr[T any](def T, p *T) T {
    if p == nil {
        return def
    }
    return *p
}
