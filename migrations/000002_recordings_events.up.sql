BEGIN;

-- Live presence, maintained by the recording-worker (and refreshed on status reads).
ALTER TABLE cameras
    ADD COLUMN IF NOT EXISTS is_online BOOLEAN NOT NULL DEFAULT FALSE;

-- ---------------------------------------------------------------------------
-- Recording segments: extra columns + uniqueness for worker upserts
-- ---------------------------------------------------------------------------
ALTER TABLE recording_segments
    ADD COLUMN IF NOT EXISTS trigger VARCHAR(32) NOT NULL DEFAULT 'continuous',
    ADD COLUMN IF NOT EXISTS mtx_path VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE recording_segments
    DROP CONSTRAINT IF EXISTS recording_segments_trigger_check;

ALTER TABLE recording_segments
    ADD CONSTRAINT recording_segments_trigger_check
        CHECK (trigger IN ('continuous', 'motion', 'manual'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_recording_segments_camera_started
    ON recording_segments (camera_id, started_at)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Events (camera online/offline, new segments, sync errors)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS events (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID         NOT NULL REFERENCES organizations (id),
    camera_id         UUID         REFERENCES cameras (id),
    type              VARCHAR(64)  NOT NULL,
    message           TEXT         NOT NULL DEFAULT '',
    metadata          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_org_created
    ON events (organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_events_camera_created
    ON events (camera_id, created_at DESC)
    WHERE camera_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_events_type
    ON events (type);

COMMIT;
