package vkbot

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ArtemST2006/VkBot/internal/vkbot/handlers"
)

const (
	defaultAPIBaseURL = "https://api.vk.com/method"
	apiVersion        = "5.199"
)

type Button = handlers.Button

type Client struct {
	token      string
	groupID    int64
	apiBaseURL string
	httpClient *http.Client
}

func NewClient(token string, groupID int64, apiBaseURL string, httpClient *http.Client) *Client {
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 40 * time.Second}
	}
	return &Client{token: token, groupID: groupID, apiBaseURL: strings.TrimRight(apiBaseURL, "/"), httpClient: httpClient}
}

type LongPollServer struct {
	Key    string `json:"key"`
	Server string `json:"server"`
	TS     string `json:"ts"`
}

func (c *Client) GetLongPollServer(ctx context.Context) (LongPollServer, error) {
	params := url.Values{"group_id": {strconv.FormatInt(c.groupID, 10)}}
	var server LongPollServer
	if err := c.call(ctx, "groups.getLongPollServer", params, &server); err != nil {
		return LongPollServer{}, err
	}
	if server.Key == "" || server.Server == "" || server.TS == "" {
		return LongPollServer{}, fmt.Errorf("VK returned incomplete Long Poll server parameters")
	}
	parsed, err := url.Parse(server.Server)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return LongPollServer{}, fmt.Errorf("VK returned invalid Long Poll server URL")
	}
	return server, nil
}

type PollResult struct {
	Updates []Update
	TS      string
	Failed  int
}

func (c *Client) GetUpdates(ctx context.Context, server LongPollServer) (PollResult, error) {
	u, err := url.Parse(server.Server)
	if err != nil {
		return PollResult{}, fmt.Errorf("parse VK Long Poll URL: %w", err)
	}
	q := u.Query()
	q.Set("act", "a_check")
	q.Set("key", server.Key)
	q.Set("ts", server.TS)
	q.Set("wait", "25")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return PollResult{}, fmt.Errorf("create VK Long Poll request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PollResult{}, fmt.Errorf("poll VK updates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return PollResult{}, fmt.Errorf("VK Long Poll returned %s", resp.Status)
	}
	var body struct {
		Updates []Update        `json:"updates"`
		TS      json.RawMessage `json:"ts"`
		Failed  int             `json:"failed"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return PollResult{}, fmt.Errorf("decode VK Long Poll response: %w", err)
	}
	ts := strings.Trim(string(body.TS), "\"")
	if body.Failed == 0 && ts == "" {
		return PollResult{}, fmt.Errorf("VK Long Poll response has no ts")
	}
	return PollResult{Updates: body.Updates, TS: ts, Failed: body.Failed}, nil
}

type vkKeyboard struct {
	OneTime bool         `json:"one_time"`
	Inline  bool         `json:"inline"`
	Buttons [][]vkButton `json:"buttons"`
}

type vkButton struct {
	Action vkAction `json:"action"`
}

type vkAction struct {
	Type    string `json:"type"`
	Label   string `json:"label"`
	Payload string `json:"payload"`
}

func (c *Client) Send(ctx context.Context, peerID int64, text string, buttons [][]Button) error {
	var randomBytes [4]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return fmt.Errorf("create VK message ID: %w", err)
	}
	randomID := binary.BigEndian.Uint32(randomBytes[:]) & 0x7fffffff
	if randomID == 0 {
		randomID = 1
	}
	params := url.Values{
		"peer_id":   {strconv.FormatInt(peerID, 10)},
		"random_id": {strconv.FormatUint(uint64(randomID), 10)},
		"message":   {text},
	}
	if len(buttons) > 0 {
		keyboard := vkKeyboard{Inline: true}
		for _, row := range buttons {
			var vkRow []vkButton
			for _, button := range row {
				payload, err := json.Marshal(struct {
					Action string `json:"action"`
				}{Action: button.Payload})
				if err != nil {
					return fmt.Errorf("encode VK button payload: %w", err)
				}
				vkRow = append(vkRow, vkButton{Action: vkAction{Type: "callback", Label: button.Text, Payload: string(payload)}})
			}
			keyboard.Buttons = append(keyboard.Buttons, vkRow)
		}
		data, err := json.Marshal(keyboard)
		if err != nil {
			return fmt.Errorf("encode VK keyboard: %w", err)
		}
		params.Set("keyboard", string(data))
	}
	var messageID int64
	return c.call(ctx, "messages.send", params, &messageID)
}

func (c *Client) Ack(ctx context.Context, eventID string, userID, peerID int64) error {
	params := url.Values{
		"event_id": {eventID},
		"user_id":  {strconv.FormatInt(userID, 10)},
		"peer_id":  {strconv.FormatInt(peerID, 10)},
	}
	var ok int
	if err := c.call(ctx, "messages.sendMessageEventAnswer", params, &ok); err != nil {
		return err
	}
	if ok != 1 {
		return fmt.Errorf("VK did not acknowledge button event")
	}
	return nil
}

func (c *Client) call(ctx context.Context, method string, params url.Values, result any) error {
	params.Set("access_token", c.token)
	params.Set("v", apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBaseURL+"/"+method, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("create VK %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call VK %s: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VK %s returned %s", method, resp.Status)
	}
	var body struct {
		Response json.RawMessage `json:"response"`
		Error    *struct {
			Code    int    `json:"error_code"`
			Message string `json:"error_msg"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return fmt.Errorf("decode VK %s response: %w", method, err)
	}
	if body.Error != nil {
		return fmt.Errorf("VK %s error %d: %s", method, body.Error.Code, body.Error.Message)
	}
	if len(body.Response) == 0 {
		return fmt.Errorf("VK %s response is empty", method)
	}
	if err := json.Unmarshal(body.Response, result); err != nil {
		return fmt.Errorf("decode VK %s result: %w", method, err)
	}
	return nil
}
