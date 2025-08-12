package util

import (
	"encoding/json"
	"io"
	"net/http"
	neturl "net/url"
	"seanime/internal/util"
	"strings"

	"github.com/labstack/echo/v4"
)

type ImageProxy struct{}

func (ip *ImageProxy) GetImage(rawURL string, headers map[string]string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	// Apply provided headers first
	for key, value := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		req.Header.Add(key, value)
	}

	// Add safe defaults if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	}
	if req.Header.Get("Accept") == "" {
		// Prefer formats we can decode: webp, png, jpeg, gif
		req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/gif,image/apng,image/*;q=0.8,*/*;q=0.5")
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}
	if req.Header.Get("Referer") == "" || req.Header.Get("Origin") == "" {
		if u, err := neturl.Parse(rawURL); err == nil {
			origin := u.Scheme + "://" + u.Host
			if req.Header.Get("Referer") == "" {
				req.Header.Set("Referer", origin+"/")
			}
			if req.Header.Get("Origin") == "" {
				req.Header.Set("Origin", origin)
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Return a descriptive error for upstream HTTP failures
		return nil, echo.NewHTTPError(resp.StatusCode, "image proxy upstream returned status "+resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (ip *ImageProxy) setHeaders(c echo.Context) {
	c.Set("Content-Type", "image/jpeg")
	c.Set("Cache-Control", "public, max-age=31536000")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET")
	c.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	c.Set("Access-Control-Allow-Credentials", "true")
}

func (ip *ImageProxy) ProxyImage(c echo.Context) (err error) {
	defer util.HandlePanicInModuleWithError("util/ImageProxy", &err)

	url := c.QueryParam("url")
	headersJSON := c.QueryParam("headers")

	if url == "" || headersJSON == "" {
		return c.String(echo.ErrBadRequest.Code, "No URL provided")
	}

	headers := make(map[string]string)
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		return c.String(echo.ErrBadRequest.Code, "Error parsing headers JSON")
	}

	ip.setHeaders(c)
	imageBuffer, err := ip.GetImage(url, headers)
	if err != nil {
		return c.String(echo.ErrInternalServerError.Code, "Error fetching image")
	}

	return c.Blob(http.StatusOK, c.Response().Header().Get("Content-Type"), imageBuffer)
}
