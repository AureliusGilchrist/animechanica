package core

import (
	"context"
	"seanime/internal/api/anilist"
	"seanime/internal/events"
	"seanime/internal/platforms/platform"
	"seanime/internal/user"
)

// GetUser returns the currently logged-in user or a simulated one.
// Note: For multi-user support, prefer GetUserFromSession when you have a session context.
func (a *App) GetUser() *user.User {
	if a.user == nil {
		return user.NewSimulatedUser()
	}
	return a.user
}

// GetUserFromSession returns the user for a specific session.
// This enables multi-user support where different browser tabs can have different accounts.
func (a *App) GetUserFromSession(sessionID string) *user.User {
	if a.SessionStore == nil {
		return a.GetUser()
	}
	sess := a.SessionStore.GetSession(sessionID)
	if sess == nil || sess.IsSimulated {
		return user.NewSimulatedUser()
	}
	return sess.ToUser()
}

func (a *App) GetUserAnilistToken() string {
	if a.user == nil || a.user.Token == user.SimulatedUserToken {
		return ""
	}

	return a.user.Token
}

// GetUserAnilistTokenFromSession returns the Anilist token for a specific session.
// This enables multi-user support where different browser tabs can have different accounts.
func (a *App) GetUserAnilistTokenFromSession(sessionID string) string {
	if a.SessionStore == nil {
		return a.GetUserAnilistToken()
	}
	sess := a.SessionStore.GetSession(sessionID)
	if sess == nil || sess.IsSimulated {
		return ""
	}
	return sess.GetToken()
}

// GetAnilistClientForSession returns the Anilist client for a specific session.
// This enables multi-user support where different browser tabs can have different accounts.
func (a *App) GetAnilistClientForSession(sessionID string) anilist.AnilistClient {
	if a.SessionStore == nil || sessionID == "" {
		return a.AnilistClientRef.Get()
	}
	return a.SessionStore.GetAnilistClient(sessionID)
}

// UpdateEntryProgressForSession updates the progress for a media entry using the session-specific Anilist client.
// This is used by PlaybackManager and DirectStreamManager to update progress for the correct user session.
// If sessionID is empty, it falls back to the global platform.
func (a *App) UpdateEntryProgressForSession(ctx context.Context, sessionID string, mediaID int, progress int, totalEpisodes *int) error {
	// If no session ID or no session store, use the global platform
	if sessionID == "" || a.SessionStore == nil {
		return a.AnilistPlatformRef.Get().UpdateEntryProgress(ctx, mediaID, progress, totalEpisodes)
	}

	// Get the session
	sess := a.SessionStore.GetSession(sessionID)
	if sess == nil || sess.IsSimulated || sess.Token == "" {
		// Fall back to global platform for simulated/unauthenticated sessions
		return a.AnilistPlatformRef.Get().UpdateEntryProgress(ctx, mediaID, progress, totalEpisodes)
	}

	// Use the session-specific Anilist client
	client := a.SessionStore.GetAnilistClient(sessionID)
	if client == nil {
		return a.AnilistPlatformRef.Get().UpdateEntryProgress(ctx, mediaID, progress, totalEpisodes)
	}

	// Determine the status based on progress
	status := anilist.MediaListStatusCurrent
	realTotalCount := 0
	if totalEpisodes != nil && *totalEpisodes > 0 {
		realTotalCount = *totalEpisodes
	}
	if realTotalCount > 0 && progress >= realTotalCount {
		status = anilist.MediaListStatusCompleted
	}
	if realTotalCount > 0 && progress > realTotalCount {
		progress = realTotalCount
	}

	_, err := client.UpdateMediaListEntryProgress(ctx, &mediaID, &progress, &status)
	return err
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// UpdatePlatform changes the current platform to the provided one.
func (a *App) UpdatePlatform(platform platform.Platform) {
	if a.AnilistPlatformRef.IsPresent() {
		a.AnilistPlatformRef.Get().Close()
	}
	a.AnilistPlatformRef.Set(platform)
	a.AddOnRefreshAnilistCollectionFunc("anilist-platform", func() {
		a.AnilistPlatformRef.Get().ClearCache()
	})
}

// UpdateAnilistClientToken will update the Anilist Client Wrapper token.
// This function should be called when a user logs in
func (a *App) UpdateAnilistClientToken(token string) {
	ac := anilist.NewAnilistClient(token, a.AnilistCacheDir)
	a.AnilistClientRef.Set(ac)
}

// GetAnimeCollection returns the user's Anilist collection if it in the cache, otherwise it queries Anilist for the user's collection.
// When bypassCache is true, it will always query Anilist for the user's collection
func (a *App) GetAnimeCollection(bypassCache bool) (*anilist.AnimeCollection, error) {
	return a.AnilistPlatformRef.Get().GetAnimeCollection(context.Background(), bypassCache)
}

// GetRawAnimeCollection is the same as GetAnimeCollection but returns the raw collection that includes custom lists
func (a *App) GetRawAnimeCollection(bypassCache bool) (*anilist.AnimeCollection, error) {
	return a.AnilistPlatformRef.Get().GetRawAnimeCollection(context.Background(), bypassCache)
}

func (a *App) SyncAnilistToSimulatedCollection() {
	if a.LocalManager != nil &&
		!a.GetUser().IsSimulated &&
		a.Settings != nil &&
		a.Settings.Library != nil &&
		a.Settings.Library.AutoSyncToLocalAccount {
		_ = a.LocalManager.SynchronizeAnilistToSimulatedCollection()
	}
}

// RefreshAnimeCollection queries Anilist for the user's collection
func (a *App) RefreshAnimeCollection() (*anilist.AnimeCollection, error) {
	go func() {
		a.OnRefreshAnilistCollectionFuncs.Range(func(key string, f func()) bool {
			go f()
			return true
		})
	}()

	ret, err := a.AnilistPlatformRef.Get().RefreshAnimeCollection(context.Background())

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

	// Save the collection to LibraryExplorer
	a.LibraryExplorer.SetAnimeCollection(ret)

	//a.SyncAnilistToSimulatedCollection()

	a.WSEventManager.SendEvent(events.RefreshedAnilistAnimeCollection, nil)

	return ret, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetMangaCollection is the same as GetAnimeCollection but for manga
func (a *App) GetMangaCollection(bypassCache bool) (*anilist.MangaCollection, error) {
	return a.AnilistPlatformRef.Get().GetMangaCollection(context.Background(), bypassCache)
}

// GetRawMangaCollection does not exclude custom lists
func (a *App) GetRawMangaCollection(bypassCache bool) (*anilist.MangaCollection, error) {
	return a.AnilistPlatformRef.Get().GetRawMangaCollection(context.Background(), bypassCache)
}

// RefreshMangaCollection queries Anilist for the user's manga collection
func (a *App) RefreshMangaCollection() (*anilist.MangaCollection, error) {
	mc, err := a.AnilistPlatformRef.Get().RefreshMangaCollection(context.Background())

	if err != nil {
		return nil, err
	}

	a.LocalManager.SetMangaCollection(mc)

	a.WSEventManager.SendEvent(events.RefreshedAnilistMangaCollection, nil)

	return mc, nil
}
