package repository

import (
	"context"
	"log"

	"fmb920-server/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool // make sure your pool is initialized somewhere

func ListLatestPositions() ([]models.LatestPosition, error) {
	rows, err := DB.Query(context.Background(), `
		SELECT DISTINCT ON (device_id)
			device_id,
			imei,
			lat AS last_lat,
			lng AS last_lng,
			timestamp AS last_position_time
		FROM positions
		WHERE timestamp >= '2000-01-01' AND timestamp <= now()
		ORDER BY device_id, timestamp DESC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []models.LatestPosition
	for rows.Next() {
		var p models.LatestPosition
		if err := rows.Scan(&p.DeviceID, &p.IMEI, &p.LastLat, &p.LastLng, &p.LastPositionTime); err != nil {
			log.Println("scan error:", err)
			continue
		}
		positions = append(positions, p)
	}

	return positions, nil
}
