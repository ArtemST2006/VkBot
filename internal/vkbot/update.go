package vkbot

import "encoding/json"

// Update contains the VK community events used by the bot.
type Update struct {
	Type   string `json:"type"`
	Object struct {
		Message *struct {
			FromID int64  `json:"from_id"`
			PeerID int64  `json:"peer_id"`
			Text   string `json:"text"`
			Out    int    `json:"out"`
		} `json:"message"`
		UserID  int64           `json:"user_id"`
		PeerID  int64           `json:"peer_id"`
		EventID string          `json:"event_id"`
		Payload json.RawMessage `json:"payload"`
	} `json:"object"`
}

func (u Update) UserID() int64 {
	if u.Type == "message_new" && u.Object.Message != nil {
		return u.Object.Message.FromID
	}
	return u.Object.UserID
}

func (u Update) PeerID() int64 {
	if u.Type == "message_new" && u.Object.Message != nil {
		return u.Object.Message.PeerID
	}
	return u.Object.PeerID
}

func (u Update) Action() string {
	var payload struct {
		Action string `json:"action"`
	}
	if json.Unmarshal(u.Object.Payload, &payload) == nil {
		return payload.Action
	}
	var encoded string
	if json.Unmarshal(u.Object.Payload, &encoded) == nil {
		if json.Unmarshal([]byte(encoded), &payload) == nil {
			return payload.Action
		}
	}
	return ""
}
