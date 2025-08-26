package handlers

import (
 "encoding/json"
 "fmt"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "time"

 "github.com/labstack/echo/v4"
 "seanime/internal/api/kitsu"
)

// NOTE: For initial implementation, we use a fixed base directory. We can move this to config later.
const lightNovelsBaseDir = "/aeternae/library/light_novels"

type LightNovelSeries struct {
 ID          string `json:"id"`
 Title       string `json:"title"`
 CoverURL    string `json:"coverUrl,omitempty"`
 VolumeCount int    `json:"volumeCount"`
}

type LightNovelVolume struct {
 FileName string `json:"fileName"`
 Path     string `json:"path"`
 Size     int64  `json:"size"`
}

type LightNovelBookmark struct {
 ID        string  `json:"id"`
 CFI       string  `json:"cfi"`
 Percent   float64 `json:"percent"`
 Label     string  `json:"label,omitempty"`
 CreatedAt int64   `json:"createdAt"`
 UpdatedAt int64   `json:"updatedAt"`
}

type lightNovelBookmarkFile struct {
 Volume    string                `json:"volume"`
 Bookmarks []LightNovelBookmark  `json:"bookmarks"`
}

// HandleGetLightNovelSeries scans the base directory and returns a list of series folders with optional covers.
func (h *Handler) HandleGetLightNovelSeries(c echo.Context) error {
 base := lightNovelsBaseDir
 entries, err := os.ReadDir(base)
 if err != nil {
  return h.RespondWithError(c, fmt.Errorf("scan light novels dir: %w", err))
 }

 var series []LightNovelSeries
 kc := kitsu.New()
 for _, e := range entries {
  if !e.IsDir() {
   continue
  }
  title := e.Name()
  // Count .epub files
  vols, _ := os.ReadDir(filepath.Join(base, title))
  count := 0
  for _, v := range vols {
   if !v.IsDir() && strings.EqualFold(filepath.Ext(v.Name()), ".epub") {
    count++
   }
  }
  cover := ""
  if title != "" {
   if u, err := kc.SearchPosterByTitle(title); err == nil {
    cover = u
   }
  }
  id := url.PathEscape(title)
  series = append(series, LightNovelSeries{ID: id, Title: title, CoverURL: cover, VolumeCount: count})
 }

 // Stable sort by title
 sort.Slice(series, func(i, j int) bool { return strings.ToLower(series[i].Title) < strings.ToLower(series[j].Title) })

 return h.RespondWithData(c, series)
}

// HandleGetLightNovelSeriesDetails lists volumes for a given series id (which encodes the folder name).
func (h *Handler) HandleGetLightNovelSeriesDetails(c echo.Context) error {
 id := c.Param("id")
 if id == "" {
  return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing id"})
 }
 title, err := url.PathUnescape(id)
 if err != nil {
  return h.RespondWithError(c, fmt.Errorf("invalid id: %w", err))
 }
 dir := filepath.Join(lightNovelsBaseDir, title)
 entries, err := os.ReadDir(dir)
 if err != nil {
  return h.RespondWithError(c, fmt.Errorf("read series dir: %w", err))
 }
 var vols []LightNovelVolume
 for _, e := range entries {
  if e.IsDir() {
   continue
  }
  if !strings.EqualFold(filepath.Ext(e.Name()), ".epub") {
   continue
  }
  p := filepath.Join(dir, e.Name())
  fi, err := os.Stat(p)
  if err != nil {
   continue
  }
  vols = append(vols, LightNovelVolume{
   FileName: e.Name(),
   Path:     p,
   Size:     fi.Size(),
  })
 }
 // Sort by natural-ish order using filename
 sort.Slice(vols, func(i, j int) bool { return strings.ToLower(vols[i].FileName) < strings.ToLower(vols[j].FileName) })
 return h.RespondWithData(c, vols)
}

// HandleServeLightNovelEPUB streams an EPUB file. Path can be sent as query param ?path=...
func (h *Handler) HandleServeLightNovelEPUB(c echo.Context) error {
 // Accept either query param or JSON body with { path: "..." }
 p := c.QueryParam("path")
 if p == "" && c.Request().Method == http.MethodPost {
  var body struct{ Path string `json:"path"` }
  if err := json.NewDecoder(c.Request().Body).Decode(&body); err == nil {
   p = body.Path
  }
 }
 if p == "" {
  return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing path"})
 }
 // Security: ensure path resides under base dir
 ap := filepath.Clean(p)
 if !strings.HasPrefix(ap, filepath.Clean(lightNovelsBaseDir)+string(os.PathSeparator)) {
  return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden path"})
 }
 if _, err := os.Stat(ap); err != nil {
  return h.RespondWithError(c, fmt.Errorf("stat epub: %w", err))
 }
 c.Response().Header().Set("Content-Type", "application/epub+zip")
 return c.Inline(ap, filepath.Base(ap))
}

// HandleGetLightNovelBookmarks lists bookmarks for a given series and volume
func (h *Handler) HandleGetLightNovelBookmarks(c echo.Context) error {
 seriesID := c.QueryParam("seriesId")
 volume := c.QueryParam("volume")
 if seriesID == "" || volume == "" {
  return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing seriesId or volume"})
 }
 title, err := url.PathUnescape(seriesID)
 if err != nil {
  return h.RespondWithError(c, fmt.Errorf("invalid seriesId: %w", err))
 }
 bf := bookmarkFilePath(title, volume)
 data, _ := os.ReadFile(bf)
 if len(data) == 0 {
  return h.RespondWithData(c, []LightNovelBookmark{})
 }
 var file lightNovelBookmarkFile
 if err := json.Unmarshal(data, &file); err != nil {
  return h.RespondWithError(c, fmt.Errorf("decode bookmarks: %w", err))
 }
 return h.RespondWithData(c, file.Bookmarks)
}

// HandleSaveLightNovelBookmark creates or updates a bookmark
func (h *Handler) HandleSaveLightNovelBookmark(c echo.Context) error {
 var req struct {
  SeriesID string  `json:"seriesId"`
  Volume   string  `json:"volume"`
  ID       string  `json:"id"`
  CFI      string  `json:"cfi"`
  Percent  float64 `json:"percent"`
  Label    string  `json:"label"`
 }
 if err := c.Bind(&req); err != nil {
  return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
 }
 if req.SeriesID == "" || req.Volume == "" || req.CFI == "" {
  return c.JSON(http.StatusBadRequest, map[string]string{"error": "seriesId, volume and cfi are required"})
 }
 title, err := url.PathUnescape(req.SeriesID)
 if err != nil {
  return h.RespondWithError(c, fmt.Errorf("invalid seriesId: %w", err))
 }
 bf := bookmarkFilePath(title, req.Volume)
 if err := os.MkdirAll(filepath.Dir(bf), 0o755); err != nil {
  return h.RespondWithError(c, fmt.Errorf("make bookmarks dir: %w", err))
 }
 var file lightNovelBookmarkFile
 if b, err := os.ReadFile(bf); err == nil && len(b) > 0 {
  _ = json.Unmarshal(b, &file)
 }
 if file.Volume == "" {
  file.Volume = req.Volume
 }
 now := time.Now().Unix()
 if req.ID == "" {
  // create
  bm := LightNovelBookmark{ID: fmt.Sprintf("%d", now), CFI: req.CFI, Percent: req.Percent, Label: req.Label, CreatedAt: now, UpdatedAt: now}
  file.Bookmarks = append(file.Bookmarks, bm)
 } else {
  // update
  updated := false
  for i := range file.Bookmarks {
   if file.Bookmarks[i].ID == req.ID {
    file.Bookmarks[i].CFI = req.CFI
    file.Bookmarks[i].Percent = req.Percent
    file.Bookmarks[i].Label = req.Label
    file.Bookmarks[i].UpdatedAt = now
    updated = true
    break
   }
  }
  if !updated {
   bm := LightNovelBookmark{ID: req.ID, CFI: req.CFI, Percent: req.Percent, Label: req.Label, CreatedAt: now, UpdatedAt: now}
   file.Bookmarks = append(file.Bookmarks, bm)
  }
 }
 out, _ := json.MarshalIndent(file, "", "  ")
 if err := os.WriteFile(bf, out, 0o644); err != nil {
  return h.RespondWithError(c, fmt.Errorf("write bookmarks: %w", err))
 }
 return h.RespondWithData(c, file.Bookmarks)
}

// HandleDeleteLightNovelBookmark deletes a bookmark by ID
func (h *Handler) HandleDeleteLightNovelBookmark(c echo.Context) error {
 var req struct {
  SeriesID string `json:"seriesId"`
  Volume   string `json:"volume"`
  ID       string `json:"id"`
 }
 if err := c.Bind(&req); err != nil {
  return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
 }
 if req.SeriesID == "" || req.Volume == "" || req.ID == "" {
  return c.JSON(http.StatusBadRequest, map[string]string{"error": "seriesId, volume and id are required"})
 }
 title, err := url.PathUnescape(req.SeriesID)
 if err != nil {
  return h.RespondWithError(c, fmt.Errorf("invalid seriesId: %w", err))
 }
 bf := bookmarkFilePath(title, req.Volume)
 var file lightNovelBookmarkFile
 if b, err := os.ReadFile(bf); err == nil && len(b) > 0 {
  _ = json.Unmarshal(b, &file)
 }
 if len(file.Bookmarks) == 0 {
  return h.RespondWithData(c, []LightNovelBookmark{})
 }
 filtered := file.Bookmarks[:0]
 for _, bm := range file.Bookmarks {
  if bm.ID != req.ID {
   filtered = append(filtered, bm)
  }
 }
 file.Bookmarks = filtered
 out, _ := json.MarshalIndent(file, "", "  ")
 if err := os.WriteFile(bf, out, 0o644); err != nil {
  return h.RespondWithError(c, fmt.Errorf("write bookmarks: %w", err))
 }
 return h.RespondWithData(c, file.Bookmarks)
}

// bookmarkFilePath returns a JSON file path for a given series title and volume filename
func bookmarkFilePath(seriesTitle, volumeFile string) string {
 safeSeries := strings.ReplaceAll(seriesTitle, string(os.PathSeparator), "_")
 safeVol := strings.ReplaceAll(volumeFile, string(os.PathSeparator), "_")
 dir := filepath.Join(lightNovelsBaseDir, ".bookmarks", safeSeries)
 return filepath.Join(dir, safeVol+".json")
}
