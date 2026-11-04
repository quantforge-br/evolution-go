// Package history stores credential-free chat events for offline desktop catch-up.
package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
)

var ErrUnavailable = errors.New("chat history storage unavailable")

const schema = `CREATE TABLE IF NOT EXISTS private_chat_events (
 id BIGSERIAL PRIMARY KEY,
 instance_id TEXT NOT NULL,
 account_jid TEXT NOT NULL,
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS private_chat_events_owner ON private_chat_events(instance_id, account_jid, id);`

const insertQuery = "INSERT INTO private_chat_events(instance_id, account_jid, payload) VALUES ($1, $2, $3)"
const readQuery = "SELECT id, payload FROM private_chat_events WHERE instance_id=$1 AND account_jid=$2 AND id>$3 ORDER BY id LIMIT $4"

// IDs are strings on the wire to avoid JavaScript integer precision loss.
type Entry struct {
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}
type Page struct {
	Events     []Entry `json:"events"`
	NextCursor string  `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}
type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }
func (s *Store) Init(ctx context.Context) error {
	if s == nil || s.db == nil {
		return ErrUnavailable
	}
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return ErrUnavailable
	}
	return nil
}

// Append commits before fan-out. The transaction-scoped lock prevents a reader
// advancing past a lower sequence ID that another writer has not committed yet.
func (s *Store) Append(ctx context.Context, instance, account, event string, data json.RawMessage, state json.RawMessage) error {
	if s == nil || s.db == nil || instance == "" || account == "" {
		return ErrUnavailable
	}
	switch event {
	case "Message", "SendMessage", "Receipt", "HistorySync":
	default:
		return nil
	}
	payload, err := json.Marshal(struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
		State json.RawMessage `json:"state,omitempty"`
	}{event, data, state})
	if err != nil {
		return ErrUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", instance+"/"+account); err != nil {
		return ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, insertQuery, instance, account, string(payload)); err != nil {
		return ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return ErrUnavailable
	}
	return nil
}

// Read never starts a WhatsApp session and never accepts an owner from a query parameter.
func (s *Store) Read(ctx context.Context, instance, account string, after int64, limit int) (Page, error) {
	page := Page{Events: make([]Entry, 0), NextCursor: strconv.FormatInt(after, 10)}
	if s == nil || s.db == nil || instance == "" || account == "" || after < 0 {
		return page, ErrUnavailable
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, readQuery, instance, account, after, limit+1)
	if err != nil {
		return page, ErrUnavailable
	}
	defer rows.Close()
	size := 0
	for rows.Next() {
		var entry Entry
		var payload []byte
		if err := rows.Scan(&entry.ID, &payload); err != nil {
			return page, ErrUnavailable
		}
		entry.Payload = json.RawMessage(payload)
		// Include a large history chunk alone, but bound ordinary page sizes to 4 MiB.
		if len(page.Events) == limit || (size+len(entry.Payload) > 4*1024*1024 && len(page.Events) > 0) {
			page.HasMore = true
			break
		}
		page.Events = append(page.Events, entry)
		page.NextCursor = entry.ID
		size += len(entry.Payload)
	}
	if rows.Err() != nil {
		return page, ErrUnavailable
	}
	return page, nil
}
