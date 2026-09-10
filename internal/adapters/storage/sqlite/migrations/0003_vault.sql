-- Phase 2: vault entries record the source commit (or registry hash) and
-- the shape they were converted from, so doctor can list conversions.

ALTER TABLE vault ADD COLUMN source_commit  TEXT NOT NULL DEFAULT '';
ALTER TABLE vault ADD COLUMN converted_from TEXT NOT NULL DEFAULT '';
