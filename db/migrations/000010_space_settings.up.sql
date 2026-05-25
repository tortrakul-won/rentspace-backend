ALTER TABLE spaces
  ADD COLUMN min_notice_hours      INTEGER NOT NULL DEFAULT 4,
  ADD COLUMN max_booking_minutes   INTEGER,
  ADD COLUMN turnaround_minutes    INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN deposit_pct           INTEGER NOT NULL DEFAULT 100;
