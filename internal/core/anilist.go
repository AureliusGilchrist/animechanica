package core

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"
	"seanime/internal/platforms/anilist_platform"
	"seanime/internal/platforms/platform"
	"seanime/internal/user"
)

// GetUser returns the currently logged-in user or a simulated one.
func (a *App) GetUser() *user.User {
	if a.user == nil {
		return user.NewSimulatedUser()
	}
	return a.user
}

// GetUserAnilistToken returns the AniList token for the current global user (deprecated - use GetUserAnilistTokenForSession)
func (a *App) GetUserAnilistToken() string {
	if a.user == nil || a.user.Token == user.SimulatedUserToken {
		return ""
	}

	return a.user.Token
}

// GetUserAnilistTokenForSession returns the AniList token for a specific session
func (a *App) GetUserAnilistTokenForSession(sessionID string) string {
	return a.SessionManager.GetToken(sessionID)
}

// GetAnilistClientForSession returns the AniList client for a specific session
func (a *App) GetAnilistClientForSession(sessionID string) anilist.AnilistClient {
	return a.SessionManager.GetClient(sessionID)
}

// IsSessionAuthenticated checks if a session is authenticated
func (a *App) IsSessionAuthenticated(sessionID string) bool {
	return a.SessionManager.IsAuthenticated(sessionID)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// UpdatePlatform changes the current platform to the provided one.
func (a *App) UpdatePlatform(platform platform.Platform) {
	a.AnilistPlatform = platform
}

// UpdateAnilistClientToken will update the Anilist Client Wrapper token.
// This function should be called when a user logs in (deprecated - use UpdateAnilistClientTokenForSession)
func (a *App) UpdateAnilistClientToken(token string) {
	a.AnilistClient = anilist.NewAnilistClient(token)
	a.AnilistPlatform.SetAnilistClient(a.AnilistClient) // Update Anilist Client Wrapper in Platform
}

// UpdateAnilistClientTokenForSession updates the AniList client for a specific session
func (a *App) UpdateAnilistClientTokenForSession(sessionID, token string) {
	// Update the session with the new token
	session, exists := a.SessionManager.GetSession(sessionID)
	if exists {
		session.Token = token
		session.Client = anilist.NewAnilistClient(token)
	}
}

// GetAnimeCollection returns the user's Anilist collection if it in the cache, otherwise it queries Anilist for the user's collection.
// When bypassCache is true, it will always query Anilist for the user's collection
func (a *App) GetAnimeCollection(bypassCache bool) (*anilist.AnimeCollection, error) {
	return a.AnilistPlatform.GetAnimeCollection(context.Background(), bypassCache)
}

// GetRawAnimeCollection is the same as GetAnimeCollection but returns the raw collection that includes custom lists
func (a *App) GetRawAnimeCollection(bypassCache bool) (*anilist.AnimeCollection, error) {
	return a.AnilistPlatform.GetRawAnimeCollection(context.Background(), bypassCache)
}

// RefreshAnimeCollection queries Anilist for the user's collection
func (a *App) RefreshAnimeCollection() (*anilist.AnimeCollection, error) {
	ret, err := a.AnilistPlatform.RefreshAnimeCollection(context.Background())

	if err != nil {
		return nil, err
	}

	// Save the collection to PlaybackManager
	a.PlaybackManager.SetAnimeCollection(ret)

	// Save the collection to AutoDownloader
	a.AutoDownloader.SetAnimeCollection(ret)

	// Save the collection to LocalManager
	a.LocalManager.SetAnimeCollection(ret)

	// Save the collection to DirectStreamManager
	a.DirectStreamManager.SetAnimeCollection(ret)

	go func() {
		for _, f := range a.OnRefreshAnilistCollectionFuncs {
			go f()
		}
	}()

	return ret, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetMangaCollection is the same as GetAnimeCollection but for manga
func (a *App) GetMangaCollection(bypassCache bool) (*anilist.MangaCollection, error) {
	return a.AnilistPlatform.GetMangaCollection(context.Background(), bypassCache)
}

// GetRawMangaCollection does not exclude custom lists
func (a *App) GetRawMangaCollection(bypassCache bool) (*anilist.MangaCollection, error) {
	return a.AnilistPlatform.GetRawMangaCollection(context.Background(), bypassCache)
}

// RefreshMangaCollection queries Anilist for the user's manga collection
func (a *App) RefreshMangaCollection() (*anilist.MangaCollection, error) {
	mc, err := a.AnilistPlatform.RefreshMangaCollection(context.Background())

	if err != nil {
		return nil, err
	}

	a.LocalManager.SetMangaCollection(mc)

	return mc, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Session-based methods for per-browser authentication

// GetAnimeCollectionForSession returns the user's Anilist collection for a specific session
func (a *App) GetAnimeCollectionForSession(sessionID string, bypassCache bool) (*anilist.AnimeCollection, error) {
	session, exists := a.SessionManager.GetSession(sessionID)
	if !exists || session.Token == "" {
		return nil, errors.New("session not found or not authenticated")
	}

	// Check cache first if not bypassing
	cacheKey := "anime_collection_" + session.Username
	if !bypassCache {
		if cached, found := a.AnilistCacheManager.GetAnimeCollection(sessionID, cacheKey); found {
			a.Logger.Debug().Str("sessionID", sessionID).Msg("Returning anime collection from cache")
			return cached, nil
		}
	}

	// Create a temporary platform for this session to fetch data
	platform := anilist_platform.NewAnilistPlatform(session.Client, a.Logger)
	platform.SetUsername(session.Username)
	
	// Use platform cache unless explicitly bypassing - this is the key performance fix
	collection, err := platform.GetAnimeCollection(context.Background(), bypassCache)
	if err != nil {
		a.Logger.Error().Err(err).Str("sessionID", sessionID).Msg("Failed to fetch anime collection")
		return nil, err
	}

	// Cache the result for faster subsequent loads
	a.AnilistCacheManager.SetAnimeCollection(sessionID, cacheKey, collection)
	a.Logger.Debug().Str("sessionID", sessionID).Msg("Cached fresh anime collection")
	return collection, nil
}

// GetMangaCollectionForSession returns the user's manga collection for a specific session
func (a *App) GetMangaCollectionForSession(sessionID string, bypassCache bool) (*anilist.MangaCollection, error) {
	session, exists := a.SessionManager.GetSession(sessionID)
	if !exists || session.Token == "" {
		return nil, errors.New("session not found or not authenticated")
	}

	// Check cache first if not bypassing
	cacheKey := "manga_collection_" + session.Username
	if !bypassCache {
		if cached, found := a.AnilistCacheManager.GetMangaCollection(sessionID, cacheKey); found {
			a.Logger.Debug().Str("sessionID", sessionID).Msg("Returning manga collection from cache")
			return cached, nil
		}
	}

	// Create a temporary platform for this session to fetch data
	platform := anilist_platform.NewAnilistPlatform(session.Client, a.Logger)
	platform.SetUsername(session.Username)
	
	// Use platform cache unless explicitly bypassing - this is the key performance fix
	collection, err := platform.GetMangaCollection(context.Background(), bypassCache)
	if err != nil {
		a.Logger.Error().Err(err).Str("sessionID", sessionID).Msg("Failed to fetch manga collection")
		return nil, err
	}

	// Cache the result for faster subsequent loads
	a.AnilistCacheManager.SetMangaCollection(sessionID, cacheKey, collection)
	a.Logger.Debug().Str("sessionID", sessionID).Msg("Cached fresh manga collection")
	return collection, nil
}

// GetRawAnimeCollectionForSession returns the user's raw Anilist collection for a specific session
func (a *App) GetRawAnimeCollectionForSession(sessionID string, bypassCache bool) (*anilist.AnimeCollection, error) {
	session, exists := a.SessionManager.GetSession(sessionID)
	if !exists || session.Token == "" {
		return nil, errors.New("session not found or not authenticated")
	}

	// Check cache first if not bypassing
	cacheKey := "raw_anime_collection_" + session.Username
	if !bypassCache {
		if cached, found := a.AnilistCacheManager.GetRawAnimeCollection(sessionID, cacheKey); found {
			a.Logger.Debug().Str("sessionID", sessionID).Msg("Returning raw anime collection from cache")
			return cached, nil
		}
	}

	// Create a temporary platform for this session to fetch data
	platform := anilist_platform.NewAnilistPlatform(session.Client, a.Logger)
	platform.SetUsername(session.Username)
	
	// Use platform cache unless explicitly bypassing - this is the key performance fix
	collection, err := platform.GetRawAnimeCollection(context.Background(), bypassCache)
	if err != nil {
		a.Logger.Error().Err(err).Str("sessionID", sessionID).Msg("Failed to fetch raw anime collection")
		return nil, err
	}

	// Cache the result for faster subsequent loads
	a.AnilistCacheManager.SetRawAnimeCollection(sessionID, cacheKey, collection)
	a.Logger.Debug().Str("sessionID", sessionID).Msg("Cached fresh raw anime collection")
	return collection, nil
}

// GetRawMangaCollectionForSession returns the user's raw manga collection for a specific session
func (a *App) GetRawMangaCollectionForSession(sessionID string, bypassCache bool) (*anilist.MangaCollection, error) {
	session, exists := a.SessionManager.GetSession(sessionID)
	if !exists || session.Token == "" {
		return nil, errors.New("session not found or not authenticated")
	}

	// Check cache first if not bypassing
	cacheKey := "raw_manga_collection_" + session.Username
	if !bypassCache {
		if cached, found := a.AnilistCacheManager.GetRawMangaCollection(sessionID, cacheKey); found {
			a.Logger.Debug().Str("sessionID", sessionID).Msg("Returning raw manga collection from cache")
			return cached, nil
		}
	}

	// Create a temporary platform for this session to fetch data
	platform := anilist_platform.NewAnilistPlatform(session.Client, a.Logger)
	platform.SetUsername(session.Username)
	
	// Use platform cache unless explicitly bypassing - this is the key performance fix
	collection, err := platform.GetRawMangaCollection(context.Background(), bypassCache)
	if err != nil {
		a.Logger.Error().Err(err).Str("sessionID", sessionID).Msg("Failed to fetch raw manga collection")
		return nil, err
	}

	// Cache the result for faster subsequent loads
	a.AnilistCacheManager.SetRawMangaCollection(sessionID, cacheKey, collection)
	a.Logger.Debug().Str("sessionID", sessionID).Msg("Cached fresh raw manga collection")
	return collection, nil
}
