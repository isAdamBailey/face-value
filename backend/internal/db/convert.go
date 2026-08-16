package db

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// ToUUID converts a uuid.UUID to the pgtype representation used by
// sqlc-generated code.
func ToUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// FromUUID converts a pgtype.UUID to uuid.UUID.
func FromUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

// ToTimestamptz converts a time.Time to the pgtype representation used by
// sqlc-generated code.
func ToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// FromTimestamptz converts a pgtype.Timestamptz to a *time.Time, returning
// nil if the value is not set.
func FromTimestamptz(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// ToNumericFromDecimal converts a decimal.Decimal to the pgtype
// representation used by sqlc-generated code, via its exact string
// representation — never through float64.
func ToNumericFromDecimal(d decimal.Decimal) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(d.String()); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// ToNumericFromFloat64 converts a float64 to the pgtype representation used
// by sqlc-generated code. Only for non-financial values (e.g. a vision
// model's self-reported confidence score) — prices must go through
// ToNumericFromDecimal instead.
func ToNumericFromFloat64(f float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// FromNumeric converts a pgtype.Numeric to a decimal.Decimal, returning
// zero for an invalid (NULL) value. Used at API response boundaries so
// prices serialize as exact decimal strings, never JSON floats.
func FromNumeric(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid || n.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(n.Int, n.Exp)
}
