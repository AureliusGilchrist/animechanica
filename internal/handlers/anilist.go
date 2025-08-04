package handlers

import (
	"errors"
	"fmt"
	"seanime/internal/api/anilist"
	"seanime/internal/events"
	"seanime/internal/platforms/anilist_platform"
	"seanime/internal/util/result"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// HandleGetAnimeCollection
//
//	@summary returns the user's AniList anime collection.
//	@desc Calling GET will return the cached anime collection.
//	@desc The manga collection is also refreshed in the background, and upon completion, a WebSocket event is sent.
//	@desc Calling POST will refetch both the anime and manga collections.
//	@returns anilist.AnimeCollection
//	@route /api/v1/anilist/collection [GET,POST]
func (h *Handler) HandleGetAnimeCollection(c echo.Context) error {

	bypassCache := c.Request().Method == "POST"

	// Get session ID from context
	sessionID := c.Get("Seanime-Client-Id").(string)

	// Check if user is authenticated
	if !h.App.SessionManager.IsAuthenticated(sessionID) {
		return h.RespondWithError(c, errors.New("not authenticated"))
	}

	// Get the user's anilist collection using session-based client
	animeCollection, err := h.App.GetAnimeCollectionForSession(sessionID, bypassCache)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	go func() {
		if h.App.Settings != nil && h.App.Settings.GetLibrary().EnableManga {
			_, _ = h.App.GetMangaCollectionForSession(sessionID, bypassCache)
			if bypassCache {
				h.App.WSEventManager.SendEvent(events.RefreshedAnilistMangaCollection, nil)
			}
		}
	}()

	return h.RespondWithData(c, animeCollection)
}

// HandleGetRawAnimeCollection
//
//	@summary returns the user's AniList anime collection without filtering out custom lists.
//	@desc Calling GET will return the cached anime collection.
//	@returns anilist.AnimeCollection
//	@route /api/v1/anilist/collection/raw [GET,POST]
func (h *Handler) HandleGetRawAnimeCollection(c echo.Context) error {

	bypassCache := c.Request().Method == "POST"

	// Get session ID from context
	sessionID := c.Get("Seanime-Client-Id").(string)

	// Check if user is authenticated
	if !h.App.SessionManager.IsAuthenticated(sessionID) {
		return h.RespondWithError(c, errors.New("not authenticated"))
	}

	// Get the user's raw anime collection using session-based client
	animeCollection, err := h.App.GetRawAnimeCollectionForSession(sessionID, bypassCache)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, animeCollection)
}

// HandleEditAnilistListEntry
//
//	@summary updates the user's list entry on Anilist.
//	@desc This is used to edit an entry on AniList.
//	@desc The "type" field is used to determine if the entry is an anime or manga and refreshes the collection accordingly.
//	@desc The client should refetch collection-dependent queries after this mutation.
//	@returns true
//	@route /api/v1/anilist/list-entry [POST]
func (h *Handler) HandleEditAnilistListEntry(c echo.Context) error {

	type body struct {
		MediaId   *int                     `json:"mediaId"`
		Status    *anilist.MediaListStatus `json:"status"`
		Score     *int                     `json:"score"`
		Progress  *int                     `json:"progress"`
		StartDate *anilist.FuzzyDateInput  `json:"startedAt"`
		EndDate   *anilist.FuzzyDateInput  `json:"completedAt"`
		Type      string                   `json:"type"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	err := h.App.AnilistPlatform.UpdateEntry(
		c.Request().Context(),
		*p.MediaId,
		p.Status,
		p.Score,
		p.Progress,
		p.StartDate,
		p.EndDate,
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	switch p.Type {
	case "anime":
		_, _ = h.App.RefreshAnimeCollection()
	case "manga":
		_, _ = h.App.RefreshMangaCollection()
	default:
		_, _ = h.App.RefreshAnimeCollection()
		_, _ = h.App.RefreshMangaCollection()
	}

	return h.RespondWithData(c, true)
}

//----------------------------------------------------------------------------------------------------------------------------------------------------

// HandleGetAnilistAnimeDetails
//
//	@summary returns more details about an AniList anime entry.
//	@desc This fetches more fields omitted from the base queries.
//	@param id - int - true - "The AniList anime ID"
//	@returns anilist.AnimeDetailsById_Media
//	@route /api/v1/anilist/media-details/{id} [GET]
func (h *Handler) HandleGetAnilistAnimeDetails(c echo.Context) error {

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Check cache first
	if cached, found := h.App.AnilistCacheManager.GetAnimeDetails(id); found {
		h.App.Logger.Debug().Int("mediaID", id).Msg("Returning anime details from cache")
		return c.JSON(200, cached)
	}

	details, err := h.App.AnilistPlatform.GetAnimeDetails(c.Request().Context(), id)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Cache the result
	h.App.AnilistCacheManager.SetAnimeDetails(id, details)

	return c.JSON(200, details)
}

//----------------------------------------------------------------------------------------------------------------------------------------------------

// HandleGetAnilistStudioDetails
//
//	@summary returns details about a studio.
//	@desc This fetches media produced by the studio.
//	@param id - int - true - "The AniList studio ID"
//	@returns anilist.StudioDetails
//	@route /api/v1/anilist/studio-details/{id} [GET]
func (h *Handler) HandleGetAnilistStudioDetails(c echo.Context) error {

	studioId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Check cache first
	if cached, found := h.App.AnilistCacheManager.GetStudio(studioId); found {
		h.App.Logger.Debug().Int("studioID", studioId).Msg("Returning studio details from cache")
		return h.RespondWithData(c, cached)
	}

	// Fetch from API
	studio, err := h.App.AnilistPlatform.GetStudioDetails(c.Request().Context(), studioId)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Cache the result
	h.App.AnilistCacheManager.SetStudio(studioId, studio)

	return h.RespondWithData(c, studio)
}

//----------------------------------------------------------------------------------------------------------------------------------------------------

// HandleDeleteAnilistListEntry
//
//	@summary deletes an entry from the user's AniList list.
//	@desc This is used to delete an entry on AniList.
//	@desc The "type" field is used to determine if the entry is an anime or manga and refreshes the collection accordingly.
//	@desc The client should refetch collection-dependent queries after this mutation.
//	@route /api/v1/anilist/list-entry [DELETE]
//	@returns bool
func (h *Handler) HandleDeleteAnilistListEntry(c echo.Context) error {

	type body struct {
		MediaId *int    `json:"mediaId"`
		Type    *string `json:"type"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	if p.Type == nil || p.MediaId == nil {
		return h.RespondWithError(c, errors.New("missing parameters"))
	}

	var listEntryID int

	switch *p.Type {
	case "anime":
		// Get the list entry ID
		animeCollection, err := h.App.GetAnimeCollection(false)
		if err != nil {
			return h.RespondWithError(c, err)
		}

		listEntry, found := animeCollection.GetListEntryFromAnimeId(*p.MediaId)
		if !found {
			return h.RespondWithError(c, errors.New("list entry not found"))
		}
		listEntryID = listEntry.ID
	case "manga":
		// Get the list entry ID
		mangaCollection, err := h.App.GetMangaCollection(false)
		if err != nil {
			return h.RespondWithError(c, err)
		}

		listEntry, found := mangaCollection.GetListEntryFromMangaId(*p.MediaId)
		if !found {
			return h.RespondWithError(c, errors.New("list entry not found"))
		}
		listEntryID = listEntry.ID
	}

	// Delete the list entry
	err := h.App.AnilistPlatform.DeleteEntry(c.Request().Context(), listEntryID)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	switch *p.Type {
	case "anime":
		_, _ = h.App.RefreshAnimeCollection()
	case "manga":
		_, _ = h.App.RefreshMangaCollection()
	}

	return h.RespondWithData(c, true)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	anilistListAnimeCache       = result.NewCache[string, *anilist.ListAnime]()
	anilistListRecentAnimeCache = result.NewCache[string, *anilist.ListRecentAnime]() // holds 1 value
)

// HandleAnilistListAnime
//
//	@summary returns a list of anime based on the search parameters.
//	@desc This is used by the "Discover" and "Advanced Search".
//	@route /api/v1/anilist/list-anime [POST]
//	@returns anilist.ListAnime
func (h *Handler) HandleAnilistListAnime(c echo.Context) error {

	type body struct {
		Page                *int                   `json:"page,omitempty"`
		Search              *string                `json:"search,omitempty"`
		PerPage             *int                   `json:"perPage,omitempty"`
		Sort                []*anilist.MediaSort   `json:"sort,omitempty"`
		Status              []*anilist.MediaStatus `json:"status,omitempty"`
		Genres              []*string              `json:"genres,omitempty"`
		AverageScoreGreater *int                   `json:"averageScore_greater,omitempty"`
		Season              *anilist.MediaSeason   `json:"season,omitempty"`
		SeasonYear          *int                   `json:"seasonYear,omitempty"`
		Format              *anilist.MediaFormat   `json:"format,omitempty"`
		IsAdult             *bool                  `json:"isAdult,omitempty"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	if p.Page == nil || p.PerPage == nil {
		*p.Page = 1
		*p.PerPage = 20
	}

	isAdult := false
	if p.IsAdult != nil {
		isAdult = *p.IsAdult && h.App.Settings.GetAnilist().EnableAdultContent
	}

	cacheKey := anilist.ListAnimeCacheKey(
		p.Page,
		p.Search,
		p.PerPage,
		p.Sort,
		p.Status,
		p.Genres,
		p.AverageScoreGreater,
		p.Season,
		p.SeasonYear,
		p.Format,
		&isAdult,
	)

	cached, ok := anilistListAnimeCache.Get(cacheKey)
	if ok {
		return h.RespondWithData(c, cached)
	}

	ret, err := anilist.ListAnimeM(
		p.Page,
		p.Search,
		p.PerPage,
		p.Sort,
		p.Status,
		p.Genres,
		p.AverageScoreGreater,
		p.Season,
		p.SeasonYear,
		p.Format,
		&isAdult,
		h.App.Logger,
		h.App.GetUserAnilistToken(),
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if ret != nil {
		anilistListAnimeCache.SetT(cacheKey, ret, time.Minute*10)
	}

	return h.RespondWithData(c, ret)
}

// HandleAnilistListRecentAiringAnime
//
//	@summary returns a list of recently aired anime.
//	@desc This is used by the "Schedule" page to display recently aired anime.
//	@route /api/v1/anilist/list-recent-anime [POST]
//	@returns anilist.ListRecentAnime
func (h *Handler) HandleAnilistListRecentAiringAnime(c echo.Context) error {

	type body struct {
		Page            *int                  `json:"page,omitempty"`
		Search          *string               `json:"search,omitempty"`
		PerPage         *int                  `json:"perPage,omitempty"`
		AiringAtGreater *int                  `json:"airingAt_greater,omitempty"`
		AiringAtLesser  *int                  `json:"airingAt_lesser,omitempty"`
		NotYetAired     *bool                 `json:"notYetAired,omitempty"`
		Sort            []*anilist.AiringSort `json:"sort,omitempty"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	if p.Page == nil || p.PerPage == nil {
		*p.Page = 1
		*p.PerPage = 50
	}

	cacheKey := fmt.Sprintf("%v-%v-%v-%v-%v-%v", p.Page, p.Search, p.PerPage, p.AiringAtGreater, p.AiringAtLesser, p.NotYetAired)

	cached, ok := anilistListRecentAnimeCache.Get(cacheKey)
	if ok {
		return h.RespondWithData(c, cached)
	}

	ret, err := anilist.ListRecentAiringAnimeM(
		p.Page,
		p.Search,
		p.PerPage,
		p.AiringAtGreater,
		p.AiringAtLesser,
		p.NotYetAired,
		p.Sort,
		h.App.Logger,
		h.App.GetUserAnilistToken(),
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	anilistListRecentAnimeCache.SetT(cacheKey, ret, time.Hour*1)

	return h.RespondWithData(c, ret)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var anilistMissedSequelsCache = result.NewCache[int, []*anilist.BaseAnime]()

// HandleAnilistListMissedSequels
//
//	@summary returns a list of sequels not in the user's list.
//	@desc This is used by the "Discover" page to display sequels the user may have missed.
//	@route /api/v1/anilist/list-missed-sequels [GET]
//	@returns []anilist.BaseAnime
func (h *Handler) HandleAnilistListMissedSequels(c echo.Context) error {

	cached, ok := anilistMissedSequelsCache.Get(1)
	if ok {
		return h.RespondWithData(c, cached)
	}

	// Get complete anime collection
	animeCollection, err := h.App.AnilistPlatform.GetAnimeCollectionWithRelations(c.Request().Context())
	if err != nil {
		return h.RespondWithError(c, err)
	}

	ret, err := anilist.ListMissedSequels(
		animeCollection,
		h.App.Logger,
		h.App.GetUserAnilistToken(),
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	anilistMissedSequelsCache.SetT(1, ret, time.Hour*4)

	return h.RespondWithData(c, ret)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// HandleGetAniListStats
//
//	@summary returns the anilist stats.
//	@desc This returns the AniList stats for the user.
//	@route /api/v1/anilist/stats [GET]
//	@returns anilist.Stats
func (h *Handler) HandleGetAniListStats(c echo.Context) error {

	// Get session ID from context
	sessionID := c.Get("Seanime-Client-Id").(string)

	// Check if user is authenticated
	if !h.App.SessionManager.IsAuthenticated(sessionID) {
		return h.RespondWithError(c, errors.New("not authenticated"))
	}

	// Check cache first
	if cached, found := h.App.AnilistCacheManager.GetViewerStats(sessionID); found {
		h.App.Logger.Debug().Str("sessionID", sessionID).Msg("Returning viewer stats from cache")
		return h.RespondWithData(c, cached)
	}

	// Get session and create temporary platform
	session, found := h.App.SessionManager.GetSession(sessionID)
	if !found {
		return h.RespondWithError(c, errors.New("session not found"))
	}

	// Create session-specific platform
	platform := anilist_platform.NewAnilistPlatform(session.Client, h.App.Logger)
	platform.SetUsername(session.Username)

	// Get viewer stats
	stats, err := platform.GetViewerStats(c.Request().Context())
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Cache the result
	h.App.AnilistCacheManager.SetViewerStats(sessionID, stats)

	return h.RespondWithData(c, stats)
}
