-- Phase 4: per-entry auto-sync flag (fan-out on every scan).

ALTER TABLE vault ADD COLUMN auto_sync INTEGER NOT NULL DEFAULT 0;
