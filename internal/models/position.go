package models

import "time"

type LatestPosition struct {
	DeviceID         int       `json:"device_id"`
	IMEI             string    `json:"imei"`
	LastLat          float64   `json:"last_lat"`
	LastLng          float64   `json:"last_lng"`
	LastPositionTime time.Time `json:"last_position_time"`
}
