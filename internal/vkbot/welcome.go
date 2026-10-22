package vkbot

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

//go:embed assets/welcome.png
var welcomeBanner []byte

func (c *Client) SendWelcome(ctx context.Context, peerID int64, text string, buttons [][]Button) error {
	attachment, err := c.uploadWelcome(ctx, peerID)
	if err != nil {
		slog.WarnContext(ctx, "VK welcome banner unavailable; sending text menu", "error", err)
		return c.Send(ctx, peerID, text, buttons)
	}
	return c.send(ctx, peerID, text, buttons, attachment, 0)
}

func (c *Client) uploadWelcome(ctx context.Context, peerID int64) (string, error) {
	var server struct {
		UploadURL string `json:"upload_url"`
	}
	if err := c.call(ctx, "photos.getMessagesUploadServer", url.Values{
		"peer_id": {strconv.FormatInt(peerID, 10)},
	}, &server); err != nil {
		return "", err
	}
	u, err := url.Parse(server.UploadURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("VK returned invalid photo upload URL")
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("photo", "welcome.png")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(welcomeBanner); err != nil {
		return "", err
	}
	if err := form.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.UploadURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload welcome photo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("VK photo upload returned %s", resp.Status)
	}
	var uploaded struct {
		Server int64  `json:"server"`
		Photo  string `json:"photo"`
		Hash   string `json:"hash"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&uploaded); err != nil {
		return "", fmt.Errorf("decode photo upload: %w", err)
	}
	if uploaded.Server == 0 || uploaded.Photo == "" || uploaded.Hash == "" {
		return "", fmt.Errorf("VK returned incomplete photo upload")
	}
	var saved []struct {
		OwnerID   int64  `json:"owner_id"`
		ID        int64  `json:"id"`
		AccessKey string `json:"access_key"`
	}
	if err := c.call(ctx, "photos.saveMessagesPhoto", url.Values{
		"server": {strconv.FormatInt(uploaded.Server, 10)},
		"photo":  {uploaded.Photo},
		"hash":   {uploaded.Hash},
	}, &saved); err != nil {
		return "", err
	}
	if len(saved) == 0 || saved[0].OwnerID == 0 || saved[0].ID == 0 {
		return "", fmt.Errorf("VK returned incomplete saved photo")
	}
	attachment := fmt.Sprintf("photo%d_%d", saved[0].OwnerID, saved[0].ID)
	if saved[0].AccessKey != "" {
		attachment += "_" + saved[0].AccessKey
	}
	return attachment, nil
}
