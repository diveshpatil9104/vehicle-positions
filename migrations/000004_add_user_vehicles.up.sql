CREATE TABLE IF NOT EXISTS user_vehicles (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vehicle_id TEXT   NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, vehicle_id)
);

CREATE INDEX IF NOT EXISTS idx_user_vehicles_vehicle_id ON user_vehicles(vehicle_id);
