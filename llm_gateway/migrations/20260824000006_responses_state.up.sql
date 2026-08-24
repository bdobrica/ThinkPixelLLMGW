CREATE TABLE responses (
    id TEXT PRIMARY KEY CHECK (id LIKE 'resp\_%'),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    previous_response_id TEXT REFERENCES responses(id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'in_progress', 'completed', 'incomplete', 'failed', 'cancelled')),
    stored BOOLEAN NOT NULL DEFAULT true,
    model TEXT NOT NULL,
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    usage JSONB,
    error JSONB,
    incomplete_details JSONB,
    provider_correlation_id TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT responses_no_self_predecessor CHECK (previous_response_id IS NULL OR previous_response_id <> id)
);

CREATE INDEX idx_responses_owner_created ON responses(api_key_id, created_at DESC);
CREATE INDEX idx_responses_owner_predecessor ON responses(api_key_id, previous_response_id) WHERE previous_response_id IS NOT NULL;
CREATE INDEX idx_responses_status_updated ON responses(status, updated_at) WHERE status IN ('queued', 'in_progress');
CREATE INDEX idx_responses_expiry ON responses(expires_at) WHERE deleted_at IS NULL;

CREATE TABLE response_items (
    response_id TEXT NOT NULL REFERENCES responses(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    direction TEXT NOT NULL CHECK (direction IN ('input', 'output')),
    item_id TEXT NOT NULL,
    item_type TEXT NOT NULL,
    status TEXT,
    call_id TEXT,
    token_count INTEGER NOT NULL DEFAULT 0 CHECK (token_count >= 0),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    encrypted_payload TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (response_id, direction, ordinal),
    UNIQUE (response_id, item_id)
);

CREATE INDEX idx_response_items_call ON response_items(response_id, call_id) WHERE call_id IS NOT NULL;

CREATE TABLE response_tool_executions (
    id TEXT PRIMARY KEY CHECK (id LIKE 'toolx\_%'),
    response_id TEXT NOT NULL REFERENCES responses(id) ON DELETE CASCADE,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    call_id TEXT NOT NULL,
    tool_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'in_progress', 'completed', 'failed', 'cancelled')),
    provider_correlation_id TEXT,
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    result JSONB,
    error JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (response_id, call_id)
);

CREATE INDEX idx_response_tool_owner_status ON response_tool_executions(api_key_id, status, updated_at);
