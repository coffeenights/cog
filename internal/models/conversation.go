package models

import (
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
)

// Message represents a single chat message
type Message struct {
	Role    string
	Content string
	Time    time.Time
}

// Conversation represents a chat conversation with messages
type Conversation struct {
	ID       string
	Name     string
	Messages []Message
	Created  time.Time
}

// FilterValue implements list.Item interface for the conversation list
func (c Conversation) FilterValue() string { return c.Name }

// Title implements list.Item interface for the conversation list
func (c Conversation) Title() string { return c.Name }

// Description implements list.Item interface for the conversation list
func (c Conversation) Description() string {
	if len(c.Messages) == 0 {
		return "New conversation"
	}
	lastMsg := c.Messages[len(c.Messages)-1]
	preview := lastMsg.Content
	if utf8.RuneCountInString(preview) > 50 {
		preview = string([]rune(preview)[:47]) + "..."
	}
	return preview
}

var _ list.Item = Conversation{}