-- name: NextDocumentSequence :one
INSERT INTO document_sequences (doc_type, year, next_seq)
VALUES ($1, $2, 2)
ON CONFLICT (doc_type, year) DO UPDATE
  SET next_seq = document_sequences.next_seq + 1
RETURNING next_seq - 1 AS seq;

-- name: CreateBookingDocument :one
INSERT INTO booking_documents (booking_id, doc_type, doc_number)
VALUES ($1, $2, $3)
ON CONFLICT (booking_id, doc_type) DO UPDATE
  SET doc_number = EXCLUDED.doc_number
RETURNING *;

-- name: GetBookingDocument :one
SELECT * FROM booking_documents
WHERE booking_id = $1 AND doc_type = $2;

-- name: ListBookingDocuments :many
SELECT * FROM booking_documents
WHERE booking_id = $1
ORDER BY issued_at ASC;
