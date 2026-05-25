CREATE TABLE space_blocks (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  space_id   UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  start_time TIMESTAMPTZ NOT NULL,
  end_time   TIMESTAMPTZ NOT NULL,
  reason     TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT valid_block_range CHECK (end_time > start_time)
);

CREATE INDEX idx_space_blocks_space ON space_blocks(space_id);
CREATE INDEX idx_space_blocks_time ON space_blocks(space_id, start_time, end_time);
