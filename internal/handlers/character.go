package handlers

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"seanime/internal/api/anilist"
)

// HandleGetCharacterDetails
//
//	@summary returns character details by ID.
//	@desc This fetches character information including all media appearances from AniList.
//	@route /api/v1/anilist/character [GET]
//	@returns *anilist.Character
func (h *Handler) HandleGetCharacterDetails(c echo.Context) error {
	// Explicit reference to ensure anilist import is recognized as used
	_ = (*anilist.Character)(nil)

	characterIdStr := c.QueryParam("id")
	if characterIdStr == "" {
		return h.RespondWithError(c, echo.NewHTTPError(400, "missing character ID"))
	}

	characterId, err := strconv.Atoi(characterIdStr)
	if err != nil {
		return h.RespondWithError(c, echo.NewHTTPError(400, "invalid character ID"))
	}

	// Check cache first
	if cached, found := h.App.AnilistCacheManager.GetCharacterDetails(characterId); found {
		h.App.Logger.Debug().Int("characterId", characterId).Msg("Returning character details from cache")
		return h.RespondWithData(c, cached)
	}

	// Fetch character details from AniList using global platform
	character, err := h.App.AnilistPlatform.CharacterDetails(c.Request().Context(), &characterId)
	if err != nil {
		h.App.Logger.Error().Err(err).Int("characterId", characterId).Msg("Failed to fetch character details")
		return h.RespondWithError(c, err)
	}

	// Cache the result
	h.App.AnilistCacheManager.SetCharacterDetails(characterId, character)

	h.App.Logger.Debug().Int("characterId", characterId).Msg("Successfully fetched character details")
	return h.RespondWithData(c, character)
}
