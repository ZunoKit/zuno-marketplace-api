package pinning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
)

// PinataClient provides minimal operations against Pinata's pinning API.
// It supports JWT or API key/secret authentication.
type PinataClient struct {
	httpClient  *http.Client
	baseURL     string
	gatewayBase string
	authHeader  string
	apiKey      string
	apiSecret   string
}

func NewPinataClient(cfg config.PinataConfig) *PinataClient {
	jwt := strings.TrimSpace(cfg.JWTKey)
	var authHeader string
	if jwt != "" {
		if !strings.HasPrefix(strings.ToLower(jwt), "bearer ") {
			authHeader = "Bearer " + jwt
		} else {
			authHeader = jwt
		}
	}
	return &PinataClient{
		httpClient:  &http.Client{Timeout: 60 * time.Second},
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		gatewayBase: strings.TrimRight(cfg.GatewayURL, "/"),
		authHeader:  authHeader,
		apiKey:      cfg.APIKey,
		apiSecret:   cfg.SecretKey,
	}
}

func (c *PinataClient) PinFile(ctx context.Context, r io.Reader, name string) (domain.PinResult, error) {
	var res domain.PinResult
	if r == nil {
		return res, errors.New("reader is nil")
	}
	if name == "" {
		name = "file"
	}

	// Add timestamp to filename to make it unique
	now := time.Now()
	ext := filepath.Ext(name)
	baseName := strings.TrimSuffix(name, ext)
	timestamp := now.Format("20060102_150405")
	uniqueName := fmt.Sprintf("%s_%s%s", baseName, timestamp, ext)

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	// write multipart in a goroutine
	go func() {
		defer pw.Close()
		defer mw.Close()

		fw, err := mw.CreateFormFile("file", filepath.Base(uniqueName))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(fw, r); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		// Optional metadata: name with timestamp
		_ = mw.WriteField("pinataMetadata", fmt.Sprintf("{\"name\":\"%s\"}", escapeJSON(uniqueName)))
	}()

	endpoint := c.baseURL + "/pinning/pinFileToIPFS"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, pr)
	if err != nil {
		return res, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.applyAuth(req)

	httpRes, err := c.httpClient.Do(req)
	if err != nil {
		return res, err
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		b, _ := io.ReadAll(httpRes.Body)
		return res, fmt.Errorf("pinata: pinFileToIPFS failed: %s: %s", httpRes.Status, strings.TrimSpace(string(b)))
	}

	dec := json.NewDecoder(httpRes.Body)
	if err := dec.Decode(&res); err != nil {
		return res, err
	}

	return res, nil
}

func (c *PinataClient) PinJSON(ctx context.Context, v any, name string) (domain.PinResult, error) {
	var res domain.PinResult
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return res, err
	}

	payload := map[string]any{
		"pinataContent": json.RawMessage(bytes.TrimSpace(buf.Bytes())),
	}
	if name != "" {
		// Add timestamp to filename to make it unique
		now := time.Now()
		ext := filepath.Ext(name)
		baseName := strings.TrimSuffix(name, ext)
		timestamp := now.Format("20060102_150405")
		uniqueName := fmt.Sprintf("%s_%s%s", baseName, timestamp, ext)
		payload["pinataMetadata"] = map[string]string{"name": uniqueName}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return res, err
	}

	endpoint := c.baseURL + "/pinning/pinJSONToIPFS"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return res, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)

	httpRes, err := c.httpClient.Do(req)
	if err != nil {
		return res, err
	}
	defer httpRes.Body.Close()
	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		b, _ := io.ReadAll(httpRes.Body)
		return res, fmt.Errorf("pinata: pinJSONToIPFS failed: %s: %s", httpRes.Status, strings.TrimSpace(string(b)))
	}
	dec := json.NewDecoder(httpRes.Body)
	if err := dec.Decode(&res); err != nil {
		return res, err
	}

	return res, nil
}

// Unpin removes a CID from Pinata.
func (c *PinataClient) Unpin(ctx context.Context, cid string) error {
	if strings.TrimSpace(cid) == "" {
		return errors.New("cid is required")
	}
	endpoint := c.baseURL + "/pinning/unpin/" + url.PathEscape(cid)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	c.applyAuth(req)
	httpRes, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpRes.Body.Close()
	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		b, _ := io.ReadAll(httpRes.Body)
		return fmt.Errorf("pinata: unpin failed: %s: %s", httpRes.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// GatewayURL composes a public gateway URL for a CID if PINATA_GATEWAY_BASE is set.
func (c *PinataClient) GatewayURL(cid string) string {
	if c.gatewayBase == "" || cid == "" {
		return ""
	}
	return c.gatewayBase + "/ipfs/" + cid
}

func (c *PinataClient) applyAuth(req *http.Request) {
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
		return
	}
	if c.apiKey != "" {
		req.Header.Set("pinata_api_key", c.apiKey)
	}
	if c.apiSecret != "" {
		req.Header.Set("pinata_secret_api_key", c.apiSecret)
	}
}

func escapeJSON(s string) string {
	// minimal escaping for inclusion in an already-JSON-quoted string
	r := strings.ReplaceAll(s, "\\", "\\\\")
	r = strings.ReplaceAll(r, "\"", "\\\"")
	r = strings.ReplaceAll(r, "\n", "\\n")
	return r
}
