CREATE TABLE IF NOT EXISTS services (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS health_checks (
    id BIGSERIAL PRIMARY KEY,
    service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('UP', 'DOWN')),
    details TEXT,
    response_time_ms BIGINT NOT NULL CHECK (response_time_ms >= 0),
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS health_checks_service_checked_at_idx
    ON health_checks (service_id, checked_at DESC);

INSERT INTO services (name, url) VALUES
    ('user-service', 'http://user-service:8081/health'),
    ('order-service', 'http://order-service:8082/health'),
    ('payment-service', 'http://payment-service:8083/health')
ON CONFLICT (name) DO UPDATE
SET url = EXCLUDED.url,
    updated_at = NOW()
WHERE (services.name = 'user-service' AND services.url = 'http://localhost:8081/health')
   OR (services.name = 'order-service' AND services.url = 'http://localhost:8082/health')
   OR (services.name = 'payment-service' AND services.url = 'http://localhost:8083/health');
