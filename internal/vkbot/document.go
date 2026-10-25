package vkbot

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
)

// SendDocument uploads a generated file to this private conversation and sends it as an attachment.
func (c *Client) SendDocument(ctx context.Context, peerID int64, filename string, contents []byte) error {
	var server struct {
		UploadURL string `json:"upload_url"`
	}
	if err := c.call(ctx, "docs.getMessagesUploadServer", url.Values{
		"peer_id": {strconv.FormatInt(peerID, 10)},
		"type":    {"doc"},
	}, &server); err != nil {
		return fmt.Errorf("get VK document upload server: %w", err)
	}
	u, err := url.Parse(server.UploadURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("VK returned invalid document upload URL")
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(contents); err != nil {
		return err
	}
	if err := form.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.UploadURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload VK document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VK document upload returned %s", resp.Status)
	}
	var uploaded struct {
		File string `json:"file"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&uploaded); err != nil {
		return fmt.Errorf("decode VK document upload: %w", err)
	}
	if uploaded.File == "" {
		return fmt.Errorf("VK returned incomplete document upload")
	}
	var saved struct {
		Doc struct {
			OwnerID   int64  `json:"owner_id"`
			ID        int64  `json:"id"`
			AccessKey string `json:"access_key"`
		} `json:"doc"`
	}
	if err := c.call(ctx, "docs.save", url.Values{
		"file":  {uploaded.File},
		"title": {filename},
	}, &saved); err != nil {
		return fmt.Errorf("save VK document: %w", err)
	}
	if saved.Doc.OwnerID == 0 || saved.Doc.ID == 0 {
		return fmt.Errorf("VK returned incomplete saved document")
	}
	attachment := fmt.Sprintf("doc%d_%d", saved.Doc.OwnerID, saved.Doc.ID)
	if saved.Doc.AccessKey != "" {
		attachment += "_" + saved.Doc.AccessKey
	}
	return c.send(ctx, peerID, "Выгрузка статистики акции", nil, attachment, 0)
}
