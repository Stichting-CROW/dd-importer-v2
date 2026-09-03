-- Migration: Add remarks column to feeds
-- Date: 2026-09-03
-- Description: Optional free-text notes for feed configuration

ALTER TABLE feeds
ADD COLUMN IF NOT EXISTS remarks TEXT;
