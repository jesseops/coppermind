ALTER TABLE works ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_works_hidden ON works(hidden);
