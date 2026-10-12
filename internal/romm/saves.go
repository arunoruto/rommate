package romm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Save sync API (RomM ≥ 4.9; device baselines and the delete action need 5.4).
// See docs: devices are registered once, then each sync is a negotiate call
// followed by uploads/downloads and a session complete.

// Version returns the server version from the heartbeat ("development" on
// dev builds).
func (c *Client) Version(ctx context.Context) (string, error) {
	var hb struct {
		System struct {
			Version string `json:"VERSION"`
		} `json:"SYSTEM"`
	}
	if err := c.getJSON(ctx, "/api/heartbeat", nil, &hb); err != nil {
		return "", err
	}
	return hb.System.Version, nil
}

// DeviceRequest registers this machine for save sync.
type DeviceRequest struct {
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Client        string `json:"client"`
	ClientVersion string `json:"client_version"`
	Hostname      string `json:"hostname"`
	SyncMode      string `json:"sync_mode"`
	AllowExisting bool   `json:"allow_existing"`
}

// RegisterDevice returns the id of the device matching hostname+platform,
// creating it if needed.
func (c *Client) RegisterDevice(ctx context.Context, req DeviceRequest) (string, error) {
	var out struct {
		DeviceID string `json:"device_id"`
	}
	if err := c.sendJSON(ctx, http.MethodPost, "/api/devices", nil, req, &out); err != nil {
		return "", err
	}
	return out.DeviceID, nil
}

// ClientSave describes a local save in a negotiate request.
type ClientSave struct {
	RomID         int       `json:"rom_id"`
	FileName      string    `json:"file_name"`
	Slot          string    `json:"slot"`
	Emulator      string    `json:"emulator,omitempty"`
	ContentHash   string    `json:"content_hash,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	FileSizeBytes int64     `json:"file_size_bytes"`
}

// NegotiateRequest asks the server what to do with each save.
type NegotiateRequest struct {
	DeviceID string       `json:"device_id"`
	RomIDs   []int        `json:"rom_ids,omitempty"` // honoured from RomM 5.3
	Saves    []ClientSave `json:"saves"`
}

// SyncOp is one decision of the server.
type SyncOp struct {
	Action            string     `json:"action"` // upload, download, conflict, no_op, delete
	RomID             int        `json:"rom_id"`
	SaveID            *int       `json:"save_id"`
	FileName          string     `json:"file_name"`
	Slot              string     `json:"slot"`
	Emulator          string     `json:"emulator"`
	Reason            string     `json:"reason"`
	ServerUpdatedAt   *time.Time `json:"server_updated_at"`
	ServerContentHash string     `json:"server_content_hash"`
}

// NegotiateResponse is the server's sync plan.
type NegotiateResponse struct {
	SessionID  int      `json:"session_id"`
	Operations []SyncOp `json:"operations"`
}

// Negotiate sends the local saves and returns the server's plan.
func (c *Client) Negotiate(ctx context.Context, req NegotiateRequest) (NegotiateResponse, error) {
	var out NegotiateResponse
	err := c.sendJSON(ctx, http.MethodPost, "/api/sync/negotiate", nil, req, &out)
	return out, err
}

// Save is the subset of RomM's SaveSchema rommate uses.
type Save struct {
	ID          int       `json:"id"`
	RomID       int       `json:"rom_id"`
	FileName    string    `json:"file_name"`
	Slot        string    `json:"slot"`
	Emulator    string    `json:"emulator"`
	ContentHash string    `json:"content_hash"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UploadParams are the query parameters of a save upload.
type UploadParams struct {
	RomID       int
	Slot        string
	Emulator    string
	DeviceID    string
	ContentHash string
	SessionID   int
	Overwrite   bool
}

// UploadSave uploads one save file (folders are uploaded as zips).
func (c *Client) UploadSave(ctx context.Context, p UploadParams, fileName string, data io.Reader) (Save, error) {
	q := url.Values{}
	q.Set("rom_id", strconv.Itoa(p.RomID))
	q.Set("slot", p.Slot)
	if p.Emulator != "" {
		q.Set("emulator", p.Emulator)
	}
	if p.DeviceID != "" {
		q.Set("device_id", p.DeviceID)
	}
	if p.ContentHash != "" {
		q.Set("content_hash", p.ContentHash)
	}
	if p.SessionID != 0 {
		q.Set("session_id", strconv.Itoa(p.SessionID))
	}
	q.Set("overwrite", strconv.FormatBool(p.Overwrite))
	q.Set("autocleanup", "true")

	// Stream the multipart body instead of buffering whole saves in memory.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, err := mw.CreateFormFile("saveFile", fileName)
		if err == nil {
			_, err = io.Copy(part, data)
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()

	req, err := c.newRequest(ctx, http.MethodPost, "/api/saves", q)
	if err != nil {
		return Save{}, err
	}
	req.Body = pr
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.do(req)
	if err != nil {
		return Save{}, err
	}
	defer resp.Body.Close()
	var out Save
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// DownloadSave streams a save's content. The caller closes the body.
func (c *Client) DownloadSave(ctx context.Context, saveID int, deviceID string, sessionID int) (io.ReadCloser, error) {
	q := url.Values{}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	if sessionID != 0 {
		q.Set("session_id", strconv.Itoa(sessionID))
	}
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/api/saves/%d/content", saveID), q)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ConfirmDownloaded records what this device wrote, as the sync baseline.
func (c *Client) ConfirmDownloaded(ctx context.Context, saveID int, deviceID, contentHash string) error {
	body := map[string]string{"device_id": deviceID, "content_hash": contentHash}
	return c.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/api/saves/%d/downloaded", saveID), nil, body, nil)
}

// CompleteSession closes a sync session with its counts.
func (c *Client) CompleteSession(ctx context.Context, sessionID, completed, failed int) error {
	body := map[string]int{"operations_completed": completed, "operations_failed": failed}
	return c.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/api/sync/sessions/%d/complete", sessionID), nil, body, nil)
}

// sendJSON sends body as JSON and decodes the response into out (if non-nil).
func (c *Client) sendJSON(ctx context.Context, method, path string, q url.Values, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, method, path, q)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(bytes.NewReader(b))
	req.ContentLength = int64(len(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
