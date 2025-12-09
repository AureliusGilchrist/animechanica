package db

import (
	"seanime/internal/database/models"
	"seanime/internal/util"
	"strings"
)

// SaveTorrentPreMatch saves a pre-match association between a destination path and media ID.
// If a pre-match already exists for the destination, it will be updated.
func (db *Database) SaveTorrentPreMatch(destination string, mediaId int) error {
	destination = util.NormalizePath(destination)

	var existing models.TorrentPreMatch
	err := db.gormdb.Where("destination = ?", destination).First(&existing).Error
	if err == nil {
		// Update existing
		existing.MediaId = mediaId
		return db.gormdb.Save(&existing).Error
	}

	// Create new
	item := &models.TorrentPreMatch{
		Destination: destination,
		MediaId:     mediaId,
	}
	return db.gormdb.Create(item).Error
}

// GetTorrentPreMatchByDestination retrieves a pre-match by destination path.
func (db *Database) GetTorrentPreMatchByDestination(destination string) (*models.TorrentPreMatch, error) {
	destination = util.NormalizePath(destination)

	var res models.TorrentPreMatch
	err := db.gormdb.Where("destination = ?", destination).First(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// GetTorrentPreMatchForFilePath checks if a file path falls under any pre-matched destination.
// Returns the media ID if found, or 0 if no pre-match exists.
func (db *Database) GetTorrentPreMatchForFilePath(filePath string) (int, bool) {
	filePath = util.NormalizePath(filePath)

	var preMatches []models.TorrentPreMatch
	err := db.gormdb.Find(&preMatches).Error
	if err != nil {
		return 0, false
	}

	for _, pm := range preMatches {
		normalizedDest := util.NormalizePath(pm.Destination)
		// Check if the file path starts with the destination path
		if strings.HasPrefix(filePath, normalizedDest) {
			return pm.MediaId, true
		}
	}

	return 0, false
}

// GetAllTorrentPreMatches retrieves all pre-match entries.
func (db *Database) GetAllTorrentPreMatches() ([]*models.TorrentPreMatch, error) {
	var res []*models.TorrentPreMatch
	err := db.gormdb.Find(&res).Error
	if err != nil {
		return nil, err
	}
	return res, nil
}

// DeleteTorrentPreMatch deletes a pre-match by ID.
func (db *Database) DeleteTorrentPreMatch(id uint) error {
	return db.gormdb.Delete(&models.TorrentPreMatch{}, id).Error
}

// DeleteTorrentPreMatchByDestination deletes a pre-match by destination path.
func (db *Database) DeleteTorrentPreMatchByDestination(destination string) error {
	destination = util.NormalizePath(destination)
	return db.gormdb.Where("destination = ?", destination).Delete(&models.TorrentPreMatch{}).Error
}

// CleanupOldTorrentPreMatches removes pre-match entries older than the specified number of days.
func (db *Database) CleanupOldTorrentPreMatches(days int) error {
	return db.gormdb.Where("created_at < datetime('now', ?)", "-"+string(rune(days))+" days").Delete(&models.TorrentPreMatch{}).Error
}

// ClearAllTorrentPreMatches removes all pre-match entries from the database.
func (db *Database) ClearAllTorrentPreMatches() error {
	return db.gormdb.Where("1 = 1").Delete(&models.TorrentPreMatch{}).Error
}
