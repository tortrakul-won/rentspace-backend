CREATE TABLE document_sequences (
  doc_type TEXT NOT NULL,
  year     INT  NOT NULL,
  next_seq INT  NOT NULL DEFAULT 1,
  PRIMARY KEY (doc_type, year)
);

CREATE TABLE booking_documents (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id UUID        NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  doc_type   TEXT        NOT NULL,
  doc_number TEXT        NOT NULL,
  issued_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (booking_id, doc_type)
);

CREATE INDEX idx_booking_documents_booking_id ON booking_documents(booking_id);
