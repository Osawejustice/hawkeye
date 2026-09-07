BEGIN;

DROP TABLE IF EXISTS events;

DROP INDEX IF EXISTS idx_recording_segments_camera_started;

ALTER TABLE recording_segments
    DROP CONSTRAINT IF EXISTS recording_segments_trigger_check;

ALTER TABLE recording_segments
    DROP COLUMN IF EXISTS trigger,
    DROP COLUMN IF EXISTS mtx_path;

ALTER TABLE cameras
    DROP COLUMN IF EXISTS is_online;

COMMIT;
