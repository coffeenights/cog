package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, err
	}

	return database, nil
}

func (d *Database) createTables() error {
	conversationsTable := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`

	messagesTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations (id) ON DELETE CASCADE
	);`

	indexTable := `
	CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at DESC);`

	for _, query := range []string{conversationsTable, messagesTable, indexTable} {
		if _, err := d.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) SaveConversation(conv Conversation) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert or update conversation
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO conversations (id, name, created_at, updated_at)
		VALUES (?, ?, ?, ?)`,
		conv.ID, conv.Name, conv.Created, time.Now())
	if err != nil {
		return err
	}

	// Delete existing messages for this conversation
	_, err = tx.Exec("DELETE FROM messages WHERE conversation_id = ?", conv.ID)
	if err != nil {
		return err
	}

	// Insert all messages
	for _, msg := range conv.Messages {
		_, err = tx.Exec(`
			INSERT INTO messages (conversation_id, role, content, created_at)
			VALUES (?, ?, ?, ?)`,
			conv.ID, msg.Role, msg.Content, msg.Time)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *Database) LoadConversations() ([]Conversation, error) {
	rows, err := d.db.Query(`
		SELECT id, name, created_at, updated_at
		FROM conversations
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		var updatedAt time.Time
		err := rows.Scan(&conv.ID, &conv.Name, &conv.Created, &updatedAt)
		if err != nil {
			return nil, err
		}

		// Load messages for this conversation
		messages, err := d.loadMessages(conv.ID)
		if err != nil {
			return nil, err
		}
		conv.Messages = messages

		conversations = append(conversations, conv)
	}

	return conversations, nil
}

func (d *Database) loadMessages(conversationID string) ([]Message, error) {
	rows, err := d.db.Query(`
		SELECT role, content, created_at
		FROM messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC`,
		conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.Role, &msg.Content, &msg.Time)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func (d *Database) DeleteConversation(conversationID string) error {
	_, err := d.db.Exec("DELETE FROM conversations WHERE id = ?", conversationID)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}