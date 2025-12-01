-- Migration: Update devices with latest coordinates from positions

BEGIN;

-- Update last_lat and last_lng in devices
UPDATE devices d
SET
    last_lat = p.lat,
    last_lng = p.lng
FROM (
    SELECT DISTINCT ON (device_id) 
        device_id,
        lat,
        lng
    FROM positions
    ORDER BY device_id, timestamp DESC
) p
WHERE d.id = p.device_id;

COMMIT;

-- Verify the update
SELECT d.id, d.imei, d.vehicle_number, d.last_lat, d.last_lng
FROM devices d
ORDER BY d.id DESC;
