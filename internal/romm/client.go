// Package romm is a small client for the RomM REST API.
package romm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client talks to a RomM server using a Client API Token (rmm_...).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	// Local reports that BaseURL is the local server address chosen by
	// NewWithFallback.
	Local bool
}

// New returns a client for the given server URL and token.
func New(baseURL, token string) *Client {
	d := &dialer{Dialer: net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		// No overall timeout: downloads can take a long time. Requests are
		// bounded by their context instead.
		HTTP: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           d.DialContext, // resolves .local names via mDNS
			ResponseHeaderTimeout: 30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		}},
	}
}

// localProbeTimeout bounds the reachability check of the local server.
const localProbeTimeout = 3 * time.Second

// NewWithFallback returns a client for local when it answers RomM's heartbeat
// quickly, and for public otherwise. With an empty local it is New(public).
func NewWithFallback(ctx context.Context, public, local, token string) *Client {
	if local != "" {
		c := New(local, token)
		probe, cancel := context.WithTimeout(ctx, localProbeTimeout)
		err := c.Heartbeat(probe)
		cancel()
		if err == nil {
			c.Local = true
			return c
		}
		if public == "" {
			return c
		}
	}
	return New(public, token)
}

// Heartbeat checks that the server is a reachable RomM instance.
func (c *Client) Heartbeat(ctx context.Context) error {
	var out map[string]any
	return c.getJSON(ctx, "/api/heartbeat", nil, &out)
}

// Platform is the subset of RomM's PlatformSchema rommate needs.
type Platform struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	FSSlug      string `json:"fs_slug"`
	Name        string `json:"name"`
	CustomName  string `json:"custom_name"`
	RomCount    int    `json:"rom_count"`
	FSSizeBytes int64  `json:"fs_size_bytes"`
}

// DisplayName prefers the user's custom platform name.
func (p Platform) DisplayName() string {
	if p.CustomName != "" {
		return p.CustomName
	}
	return p.Name
}

// RomFile is one file belonging to a ROM.
type RomFile struct {
	ID            int    `json:"id"`
	FileName      string `json:"file_name"`
	FilePath      string `json:"file_path"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	IsTopLevel    bool   `json:"is_top_level"`
}

// Rom is the subset of RomM's SimpleRomSchema rommate needs.
type Rom struct {
	ID               int       `json:"id"`
	PlatformID       int       `json:"platform_id"`
	PlatformSlug     string    `json:"platform_slug"`
	PlatformFSSlug   string    `json:"platform_fs_slug"`
	Name             string    `json:"name"`
	FSName           string    `json:"fs_name"`
	FSSizeBytes      int64     `json:"fs_size_bytes"`
	HasMultipleFiles bool      `json:"has_multiple_files"`
	Files            []RomFile `json:"files"`
}

// Title returns the matched game name, falling back to the file name.
func (r Rom) Title() string {
	if r.Name != "" {
		return r.Name
	}
	return r.FSName
}

type page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("romm: HTTP %d: %s", e.Status, e.Body)
}

// ErrUnauthorized means the token is missing, invalid or lacks scopes.
var ErrUnauthorized = errors.New("romm: unauthorized (check your API token)")

func (c *Client) newRequest(ctx context.Context, method, path string, q url.Values) (*http.Request, error) {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("User-Agent", "rommate")
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrUnauthorized
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return nil, &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
}

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, q)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// Platforms lists all platforms on the server.
func (c *Client) Platforms(ctx context.Context) ([]Platform, error) {
	var out []Platform
	err := c.getJSON(ctx, "/api/platforms", nil, &out)
	return out, err
}

// Roms lists every ROM of a platform, following pagination.
func (c *Client) Roms(ctx context.Context, platformID int) ([]Rom, error) {
	const limit = 500
	var all []Rom
	for offset := 0; ; offset += limit {
		q := url.Values{}
		q.Set("platform_ids", strconv.Itoa(platformID))
		q.Set("with_files", "true")
		q.Set("order_by", "name")
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))
		var p page[Rom]
		if err := c.getJSON(ctx, "/api/roms", q, &p); err != nil {
			return nil, err
		}
		all = append(all, p.Items...)
		if len(p.Items) < limit {
			return all, nil
		}
	}
}

// Rom fetches a single ROM by id.
func (c *Client) Rom(ctx context.Context, id int) (Rom, error) {
	var r Rom
	err := c.getJSON(ctx, "/api/roms/"+strconv.Itoa(id), nil, &r)
	return r, err
}

// OpenFile starts downloading one file of a ROM. If offset > 0 a Range request
// is made; the returned bool reports whether the server honoured it (206).
// The caller must close the returned body.
func (c *Client) OpenFile(ctx context.Context, romID int, f RomFile, offset int64) (io.ReadCloser, int64, bool, error) {
	q := url.Values{}
	q.Set("file_ids", strconv.Itoa(f.ID))
	path := "/api/roms/" + strconv.Itoa(romID) + "/content/" + url.PathEscape(f.FileName)
	req, err := c.newRequest(ctx, http.MethodGet, path, q)
	if err != nil {
		return nil, 0, false, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, 0, false, err
	}
	resumed := resp.StatusCode == http.StatusPartialContent
	return resp.Body, resp.ContentLength, resumed, nil
}
