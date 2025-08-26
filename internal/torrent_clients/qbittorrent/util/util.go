package qbittorrent_util

import (
	"bytes"
	"fmt"
	stdjson "encoding/json" // stdlib json
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"

	qbittorrent_model "seanime/internal/torrent_clients/qbittorrent/model"

	"github.com/goccy/go-json"
)

func GetInto(client *http.Client, target interface{}, url string, body interface{}) (err error) {
	var buffer bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buffer).Encode(body); err != nil {
			return err
		}
	}
	r, err := http.NewRequest("GET", url, &buffer)
	if err != nil {
		return err
	}
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := resp.Body.Close(); err2 != nil {
			err = err2
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response status %s", resp.Status)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.NewDecoder(bytes.NewReader(buf)).Decode(target); err != nil {
		// Fallback 1: sanitize NaN/Infinity values sometimes returned by qBittorrent
		sanitized := sanitizeNonStandardNumbers(buf)
		if !bytes.Equal(sanitized, buf) {
			if err3 := json.NewDecoder(bytes.NewReader(sanitized)).Decode(target); err3 == nil {
				return nil
			}
			// Try stdlib decoder on sanitized buffer
			if err4 := stdjson.NewDecoder(bytes.NewReader(sanitized)).Decode(target); err4 == nil {
				return nil
			}
		}
		// Try stdlib decoder on original buffer
		if err5 := stdjson.NewDecoder(bytes.NewReader(buf)).Decode(target); err5 == nil {
			return nil
		}
		// Fallback 2: endpoints that return a raw string (e.g., version)
		ct := resp.Header.Get("Content-Type")
		snippet := string(buf)
		if len(snippet) > 512 {
			snippet = snippet[:512] + "..."
		}
		if err2 := json.NewDecoder(strings.NewReader(`"` + string(buf) + `"`)).Decode(target); err2 != nil {
			return fmt.Errorf("qbittorrent_util.GetInto: failed to decode JSON: %v (status=%s, content-type=%s) body_snippet=%q", err, resp.Status, ct, snippet)
		}
	}
	return nil
}

// sanitizeNonStandardNumbers replaces non-standard JSON numeric tokens with null.
// qBittorrent may return NaN/Infinity for some float fields which is invalid JSON.
// We perform a conservative replacement outside of strings by looking for colon-value patterns.
func sanitizeNonStandardNumbers(b []byte) []byte {
	// Regex: colon, optional spaces, then NaN or +/-Infinity
	// Replace with ": null"
	re := regexp.MustCompile(`:\s*(NaN|Infinity|-Infinity)`) // best-effort
	return re.ReplaceAll(b, []byte(`: null`))
}

func Post(client *http.Client, url string, body interface{}) (err error) {
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(body); err != nil {
		return err
	}
	r, err := http.NewRequest("POST", url, &buffer)
	if err != nil {
		return err
	}
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := resp.Body.Close(); err2 != nil {
			err = err2
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status %s", resp.Status)
	}
	return nil
}

func createFormFileWithHeader(writer *multipart.Writer, name, filename string, headers map[string]string) (io.Writer, error) {
	header := textproto.MIMEHeader{}
	header.Add("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, name, filename))
	for key, value := range headers {
		header.Add(key, value)
	}
	return writer.CreatePart(header)
}

func PostMultipartLinks(client *http.Client, url string, options *qbittorrent_model.AddTorrentsOptions, links []string) (err error) {
	var o map[string]interface{}
	if options != nil {
		b, err := json.Marshal(options)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &o); err != nil {
			return err
		}
	}
	buf := bytes.Buffer{}
	form := multipart.NewWriter(&buf)
	if err := form.WriteField("urls", strings.Join(links, "\n")); err != nil {
		return err
	}
	for key, value := range o {
		if err := form.WriteField(key, fmt.Sprintf("%v", value)); err != nil {
			return err
		}
	}
	if err := form.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "multipart/form-data; boundary="+form.Boundary())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := resp.Body.Close(); err2 != nil {
			err = err2
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status %s", resp.Status)
	}
	return nil
}

func PostMultipartFiles(client *http.Client, url string, options *qbittorrent_model.AddTorrentsOptions, files map[string][]byte) (err error) {
	var o map[string]interface{}
	if options != nil {
		b, err := json.Marshal(options)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &o); err != nil {
			return err
		}
	}
	buf := bytes.Buffer{}
	form := multipart.NewWriter(&buf)
	for filename, file := range files {
		writer, err := createFormFileWithHeader(form, "torrents", filename, map[string]string{
			"content-type": "application/x-bittorrent",
		})
		if err != nil {
			return err
		}
		if _, err := writer.Write(file); err != nil {
			return err
		}
	}
	for key, value := range o {
		if err := form.WriteField(key, fmt.Sprintf("%v", value)); err != nil {
			return err
		}
	}
	if err := form.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "multipart/form-data; boundary="+form.Boundary())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := resp.Body.Close(); err2 != nil {
			err = err2
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status %s", resp.Status)
	}
	return nil
}

func PostWithContentType(client *http.Client, url string, body io.Reader, contentType string) (err error) {
	r, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	r.Header.Add("content-type", contentType)
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := resp.Body.Close(); err2 != nil {
			err = err2
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status %s", resp.Status)
	}
	return nil
}

func GetIntoWithContentType(client *http.Client, target interface{}, url string, body io.Reader, contentType string) (err error) {
	r, err := http.NewRequest("GET", url, body)
	if err != nil {
		return err
	}
	r.Header.Add("content-type", contentType)
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := resp.Body.Close(); err2 != nil {
			err = err2
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response status %s", resp.Status)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.NewDecoder(bytes.NewReader(buf)).Decode(target); err != nil {
		// Fallback 1: sanitize invalid JSON numeric tokens
		sanitized := sanitizeNonStandardNumbers(buf)
		if !bytes.Equal(sanitized, buf) {
			if err3 := json.NewDecoder(bytes.NewReader(sanitized)).Decode(target); err3 == nil {
				return nil
			}
			if err4 := stdjson.NewDecoder(bytes.NewReader(sanitized)).Decode(target); err4 == nil {
				return nil
			}
		}
		if err5 := stdjson.NewDecoder(bytes.NewReader(buf)).Decode(target); err5 == nil {
			return nil
		}
		// Fallback 2: raw string endpoints
		if err2 := json.NewDecoder(strings.NewReader(`"` + string(buf) + `"`)).Decode(target); err2 != nil {
			return err
		}
	}
	return nil
}
