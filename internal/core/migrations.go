package core

import (
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-json"
	"github.com/samber/mo"
	"seanime/internal/constants"
	"seanime/internal/database/db_bridge"
	"seanime/internal/database/models"
	"seanime/internal/library/anime"
	"seanime/internal/util"
	"strings"
	"time"
)

func (a *App) runMigrations() {

	go func() {
		done := false
		defer func() {
			if done {
				a.Logger.Info().Msg("app: Version migration complete")
			}
		}()
		defer util.HandlePanicThen(func() {
			a.Logger.Error().Msg("app: runMigrations failed")
		})

		previousVersion, err := semver.NewVersion(a.previousVersion)
		if err != nil {
			a.Logger.Error().Err(err).Msg("app: Failed to parse previous version")
			return
		}

		if a.previousVersion != constants.Version {

			hasUpdated := util.VersionIsOlderThan(a.previousVersion, constants.Version)

			//-----------------------------------------------------------------------------------------
			// DEVNOTE: 1.2.0 uses an incorrect manga cache format for MangaSee pages
			// This migration will remove all manga cache files that start with "manga_"
			if a.previousVersion == "1.2.0" && hasUpdated {
				a.Logger.Debug().Msg("app: Executing version migration task")
				err := a.FileCacher.RemoveAllBy(func(filename string) bool {
					return strings.HasPrefix(filename, "manga_")
				})
				if err != nil {
					a.Logger.Error().Err(err).Msg("app: MIGRATION FAILED; READ THIS")
					a.Logger.Error().Msg("app: Failed to remove 'manga' cache files, please clear them manually by going to the settings. Ignore this message if you have no manga cache files.")
				}
				done = true
			}

			//-----------------------------------------------------------------------------------------

			c1, _ := semver.NewConstraint("<= 1.3.0, >= 1.2.0")
			if c1.Check(previousVersion) {
				a.Logger.Debug().Msg("app: Executing version migration task")
				err := a.FileCacher.RemoveAllBy(func(filename string) bool {
					return strings.HasPrefix(filename, "manga_")
				})
				if err != nil {
					a.Logger.Error().Err(err).Msg("app: MIGRATION FAILED; READ THIS")
					a.Logger.Error().Msg("app: Failed to remove 'manga' cache files, please clear them manually by going to the settings. Ignore this message if you have no manga cache files.")
				}
				done = true
			}

			//-----------------------------------------------------------------------------------------

			// DEVNOTE: 1.5.6 uses a different cache format for media streaming info
			// -> Delete the cache files when updated from any version between 1.5.0 and 1.5.5
			c2, _ := semver.NewConstraint("<= 1.5.5, >= 1.5.0")
			if c2.Check(previousVersion) {
				a.Logger.Debug().Msg("app: Executing version migration task")
				err := a.FileCacher.RemoveAllBy(func(filename string) bool {
					return strings.HasPrefix(filename, "mediastream_mediainfo_")
				})
				if err != nil {
					a.Logger.Error().Err(err).Msg("app: MIGRATION FAILED; READ THIS")
					a.Logger.Error().Msg("app: Failed to remove transcoding cache files, please clear them manually by going to the settings. Ignore this message if you have no transcoding cache files.")
				}
				done = true
			}

			//-----------------------------------------------------------------------------------------

			// DEVNOTE: 2.0.0 uses a different cache format for online streaming
			// -> Delete the cache files when updated from a version older than 2.0.0 and newer than 1.5.0
			c3, _ := semver.NewConstraint("< 2.0.0, >= 1.5.0")
			if c3.Check(previousVersion) {
				a.Logger.Debug().Msg("app: Executing version migration task")
				err := a.FileCacher.RemoveAllBy(func(filename string) bool {
					return strings.HasPrefix(filename, "onlinestream_")
				})
				if err != nil {
					a.Logger.Error().Err(err).Msg("app: MIGRATION FAILED; READ THIS")
					a.Logger.Error().Msg("app: Failed to remove online streaming cache files, please clear them manually by going to the settings. Ignore this message if you have no online streaming cache files.")
				}
				done = true
			}

			//-----------------------------------------------------------------------------------------

			// DEVNOTE: 2.1.0 refactored the manga cache format
			// -> Delete the cache files when updated from a version older than 2.1.0
			c4, _ := semver.NewConstraint("< 2.1.0")
			if c4.Check(previousVersion) {
				a.Logger.Debug().Msg("app: Executing version migration task")
				err := a.FileCacher.RemoveAllBy(func(filename string) bool {
					return strings.HasPrefix(filename, "manga_")
				})
				if err != nil {
					a.Logger.Error().Err(err).Msg("app: MIGRATION FAILED; READ THIS")
					a.Logger.Error().Msg("app: Failed to remove 'manga' cache files, please clear them manually by going to the settings. Ignore this message if you have no manga cache files.")
				}
				done = true
			}

			//-----------------------------------------------------------------------------------------

			// Consolidate LocalFiles into a single global collection.
			// Rationale: older builds might have created multiple LocalFiles rows (e.g., per profile or due to duplication).
			// Strategy: merge all rows into the newest row, resolving conflicts per path by:
			//   1) Prefer locked=true over locked=false
			//   2) If lock state is equal, prefer the newest row (latest UpdatedAt/ID)
			{
				list, err := a.Database.ListAllLocalFilesModels()
				if err != nil {
					a.Logger.Error().Err(err).Msg("app: Failed to list LocalFiles models for consolidation")
				} else if len(list) > 1 {
					a.Logger.Debug().Int("count", len(list)).Msg("app: Consolidating LocalFiles into a single global store")

					type withStamp struct {
						lf    *anime.LocalFile
						stamp time.Time
						srcID uint
					}
					merged := make(map[string]withStamp) // key by absolute path

					for _, row := range list {
						var files []*anime.LocalFile
						if uerr := json.Unmarshal(row.Value, &files); uerr != nil {
							a.Logger.Error().Err(uerr).Uint("rowId", row.ID).Msg("app: Failed to unmarshal LocalFiles row; skipping")
							continue
						}
						for _, f := range files {
							if f == nil || f.Path == "" {
								continue
							}
							if curr, ok := merged[f.Path]; ok {
								// Conflict resolution
								candLocked := f.Locked
								currLocked := curr.lf != nil && curr.lf.Locked
								replace := false
								switch {
								case candLocked && !currLocked:
									replace = true
								case candLocked == currLocked:
									// Prefer newer row by UpdatedAt (fall back to higher ID)
									newer := row.UpdatedAt.After(curr.stamp) || (row.UpdatedAt.Equal(curr.stamp) && row.ID > curr.srcID)
									if newer {
										replace = true
									}
								}
								if replace {
									merged[f.Path] = withStamp{lf: f, stamp: row.UpdatedAt, srcID: row.ID}
								}
							} else {
								merged[f.Path] = withStamp{lf: f, stamp: row.UpdatedAt, srcID: row.ID}
							}
						}
					}

					// Build merged array
					out := make([]*anime.LocalFile, 0, len(merged))
					for _, v := range merged {
						out = append(out, v.lf)
					}

					// Upsert into the newest row (highest ID), then delete the rest
					newest := list[len(list)-1]
					marshaled, jerr := json.Marshal(out)
					if jerr != nil {
						a.Logger.Error().Err(jerr).Msg("app: Failed to marshal merged LocalFiles; aborting consolidation")
					} else {
						newest.Value = marshaled
						if _, uerr := a.Database.UpsertLocalFiles(newest); uerr != nil {
							a.Logger.Error().Err(uerr).Msg("app: Failed to upsert consolidated LocalFiles")
						} else {
							// Remove all other rows, keep only the newest one
							if derr := a.Database.Gorm().Where("id <> ?", newest.ID).Delete(&models.LocalFiles{}).Error; derr != nil {
								a.Logger.Error().Err(derr).Msg("app: Failed to delete old LocalFiles rows after consolidation")
							} else {
								// Also refresh db_bridge cache since IDs/values changed
								db_bridge.CurrLocalFiles = mo.None[[]*anime.LocalFile]()
								db_bridge.CurrLocalFilesDbId = newest.ID
								a.Logger.Info().Int("merged", len(out)).Msg("app: LocalFiles consolidation complete")
								done = true
							}
						}
					}
				}
			}
		}
	}()

}
