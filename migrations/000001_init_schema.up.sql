BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- Organizations (tenancy boundary)
-- ---------------------------------------------------------------------------
CREATE TABLE organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_organizations_slug_active
    ON organizations (slug)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_organizations_deleted_at ON organizations (deleted_at);

-- ---------------------------------------------------------------------------
-- Users
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID         NOT NULL REFERENCES organizations (id),
    email            VARCHAR(255) NOT NULL,
    password_hash    VARCHAR(255) NOT NULL,
    name             VARCHAR(255) NOT NULL,
    role             VARCHAR(32)  NOT NULL DEFAULT 'admin',
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    last_login_at    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT users_role_check CHECK (role IN ('admin', 'user'))
);

CREATE UNIQUE INDEX idx_users_email_active
    ON users (LOWER(email))
    WHERE deleted_at IS NULL;

CREATE INDEX idx_users_organization_id
    ON users (organization_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_users_deleted_at ON users (deleted_at);

-- ---------------------------------------------------------------------------
-- Refresh tokens (hashed, rotatable)
-- ---------------------------------------------------------------------------
CREATE TABLE refresh_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   VARCHAR(64) NOT NULL,
    user_agent   TEXT,
    ip_address   VARCHAR(64),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    replaced_by  UUID        REFERENCES refresh_tokens (id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- Cameras
-- ---------------------------------------------------------------------------
CREATE TABLE cameras (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id    UUID         NOT NULL REFERENCES organizations (id),
    owner_id           UUID         NOT NULL REFERENCES users (id),
    name               VARCHAR(255) NOT NULL,
    description        TEXT         NOT NULL DEFAULT '',
    location           VARCHAR(255) NOT NULL DEFAULT '',
    rtsp_url           TEXT         NOT NULL,
    rtsp_username      VARCHAR(255) NOT NULL DEFAULT '',
    rtsp_password      TEXT         NOT NULL DEFAULT '',
    enabled            BOOLEAN      NOT NULL DEFAULT TRUE,
    recording_enabled  BOOLEAN      NOT NULL DEFAULT FALSE,
    mtx_path           VARCHAR(255) NOT NULL,
    mtx_sync_status    VARCHAR(32)  NOT NULL DEFAULT 'pending',
    mtx_sync_error     TEXT         NOT NULL DEFAULT '',
    last_seen_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    CONSTRAINT cameras_mtx_sync_status_check
        CHECK (mtx_sync_status IN ('pending', 'synced', 'error', 'skipped'))
);

CREATE UNIQUE INDEX idx_cameras_mtx_path_active
    ON cameras (mtx_path)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_cameras_org_name_active
    ON cameras (organization_id, LOWER(name))
    WHERE deleted_at IS NULL;

CREATE INDEX idx_cameras_owner_id
    ON cameras (owner_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_cameras_organization_id
    ON cameras (organization_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_cameras_deleted_at ON cameras (deleted_at);

-- ---------------------------------------------------------------------------
-- Recording segments (metadata only — bytes live in storage)
-- ---------------------------------------------------------------------------
CREATE TABLE recording_segments (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID         NOT NULL REFERENCES organizations (id),
    camera_id         UUID         NOT NULL REFERENCES cameras (id),
    started_at        TIMESTAMPTZ  NOT NULL,
    ended_at          TIMESTAMPTZ,
    duration_ms       BIGINT,
    storage_backend   VARCHAR(32)  NOT NULL DEFAULT 'local',
    storage_path      TEXT         NOT NULL,
    size_bytes        BIGINT,
    format            VARCHAR(32)  NOT NULL DEFAULT 'fmp4',
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX idx_recording_segments_camera_started
    ON recording_segments (camera_id, started_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_recording_segments_organization_id
    ON recording_segments (organization_id)
    WHERE deleted_at IS NULL;

COMMIT;
