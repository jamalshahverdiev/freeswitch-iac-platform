-- mod_conference desired state: profiles (settings groups) and rooms.
-- A room materializes both a conference and the dialplan extension that
-- callers dial to enter it.

CREATE TABLE IF NOT EXISTS conference_profiles (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,                 -- e.g. video-grid
    rate INTEGER NOT NULL DEFAULT 48000,
    interval_ms INTEGER NOT NULL DEFAULT 20,
    energy_level INTEGER NOT NULL DEFAULT 200,
    comfort_noise BOOLEAN NOT NULL DEFAULT TRUE,
    moh_sound TEXT NOT NULL DEFAULT 'local_stream://moh',
    video_mode TEXT NOT NULL DEFAULT '',       -- '' = audio only, 'mux' = video conference
    video_layout TEXT NOT NULL DEFAULT 'group:grid',
    video_canvas_size TEXT NOT NULL DEFAULT '1280x720',
    video_fps INTEGER NOT NULL DEFAULT 15,
    auto_record TEXT NOT NULL DEFAULT '',      -- path template; '' = no recording
    params JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS conference_rooms (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,                 -- e.g. standup
    number TEXT NOT NULL,                      -- dialable, e.g. 3500
    domain TEXT NOT NULL,
    context TEXT NOT NULL,
    profile TEXT NOT NULL REFERENCES conference_profiles(name) ON DELETE RESTRICT ON UPDATE CASCADE,
    pin TEXT NOT NULL DEFAULT '',
    max_members INTEGER NOT NULL DEFAULT 0,    -- 0 = unlimited
    priority INTEGER NOT NULL DEFAULT 5,       -- dialplan ordering (lower wins)
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(context, number)
);

CREATE INDEX IF NOT EXISTS idx_conference_rooms_context ON conference_rooms(context);
