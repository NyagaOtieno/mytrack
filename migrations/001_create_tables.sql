CREATE TABLE IF NOT EXISTS devices (
  imei VARCHAR(20) PRIMARY KEY,
  sim_number VARCHAR(50),
  vehicle_number VARCHAR(50),
  chassis VARCHAR(100),
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS positions (
  id BIGSERIAL PRIMARY KEY,
  imei VARCHAR(20) REFERENCES devices(imei),
  ts TIMESTAMPTZ NOT NULL,
  lat DOUBLE PRECISION,
  lon DOUBLE PRECISION,
  speed DOUBLE PRECISION,
  raw BYTEA,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_positions_imei_ts ON positions(imei, ts DESC);
