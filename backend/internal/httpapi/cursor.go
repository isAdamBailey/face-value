package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// cursorPayload is the opaque cursor's decoded form: keyset pagination on
// (created_at, id), never OFFSET — OFFSET pagination duplicates and shifts
// rows the moment a new search is inserted above the cursor mid-scroll.
type cursorPayload struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"i"`
}

func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(cursorPayload{CreatedAt: createdAt, ID: id}) //nolint:errchkjson // struct always marshals
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decode cursor: %w", err)
	}
	var c cursorPayload
	if err := json.Unmarshal(b, &c); err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("unmarshal cursor: %w", err)
	}
	return c.CreatedAt, c.ID, nil
}
