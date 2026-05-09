package memory

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

// uuidToPg converts a uuid.UUID to pgtype.UUID for SQL parameter binding.
func uuidToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(u), Valid: true}
}

// vectorToPg converts a []float32 to pgvector.Vector for SQL parameter binding.
func vectorToPg(vec []float32) pgvector.Vector {
	return pgvector.NewVector(vec)
}

// boolToPg converts a bool to pgtype.Bool for SQL parameter binding.
func boolToPg(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}