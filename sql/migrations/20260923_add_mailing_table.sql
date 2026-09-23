-- Migration: Create mailing table
-- Date: 2026-09-23
-- Description: Table to store sent mailings (subject, body and recipient filters)

BEGIN;

CREATE TABLE IF NOT EXISTS mailing (
    mailing_id                  SERIAL PRIMARY KEY,
    subject                     TEXT NOT NULL,
    body_markdown               TEXT NOT NULL,
    filter_organisation_id      INT,
    filter_core_group_only      BOOLEAN NOT NULL DEFAULT false,
    filter_microhub_edit_only   BOOLEAN NOT NULL DEFAULT false,
    is_test                     BOOLEAN NOT NULL DEFAULT false,
    recipient_count             INT NOT NULL,
    sent_by                     VARCHAR(255) NOT NULL,
    sent_at                     TIMESTAMP NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE ON TABLE mailing TO dashboarddeelmobiliteit;
GRANT SELECT, USAGE ON SEQUENCE mailing_mailing_id_seq TO dashboarddeelmobiliteit;

COMMIT;