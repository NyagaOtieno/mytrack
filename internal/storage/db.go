package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

/* ---------------------- GLOBALS ---------------------- */

var (
	Pool       *pgxpool.Pool
	backendURL string
	httpClient = &http.Client{Timeout: 10 * time.Second}
)

/* ---------------------- INIT ---------------------- */

func InitDB(connStr string) error {
	var err error
	Pool, err = pgxpool.New(context.Background(), connStr)
	if err != nil {
		return err
	}
	return Pool.Ping(context.Background())
}

func SetBackendURL(url string) {
	backendURL = url
}

/* ---------------------- STRUCTS ---------------------- */

type DeviceWithVehicle struct {
	DeviceID      int64    `json:"id"`
	IMEI          string   `json:"imei"`
	SIM           string   `json:"sim"`
	VehicleID     *int64   `json:"vehicle_id,omitempty"`
	VehicleNumber *string  `json:"vehicle_no,omitempty"`
	ChassisNo     *string  `json:"chassis_no,omitempty"`
	OwnerName     *string  `json:"owner_name,omitempty"`
	OwnerID       *string  `json:"owner_id_number,omitempty"`
	OwnerPhone    *string  `json:"owner_phone,omitempty"`
	OwnerEmail    *string  `json:"owner_email,omitempty"`
	LastLat       *float64 `json:"last_lat,omitempty"`
	LastLng       *float64 `json:"last_lng,omitempty"`
}

type Device struct {
	ID        int64
	IMEI      string
	SIM       string
	VehicleNo string
	ChassisNo string
	LastLat   *float64
	LastLng   *float64
}

type Position struct {
	DeviceID   int64
	IMEI       string
	Latitude   float64
	Longitude  float64
	Speed      float64
	Angle      float64
	Altitude   float64
	Satellites int
	Timestamp  time.Time
}

/* ---------------------- DEVICE HELPERS ---------------------- */

func GetDeviceIDByIMEI(imei string) (int64, error) {
	var id int64
	err := Pool.QueryRow(context.Background(),
		"SELECT id FROM devices WHERE imei=$1", imei,
	).Scan(&id)
	return id, err
}

func UpdateDeviceLastPosition(deviceID int64, lat, lng float64) error {
	_, err := Pool.Exec(context.Background(),
		"UPDATE devices SET last_lat=$1, last_lng=$2 WHERE id=$3",
		lat, lng, deviceID,
	)
	return err
}

/* ---------------------- DEVICE CREATION ---------------------- */

func insertDeviceLocally(ctx context.Context, d Device) (int64, error) {
	var id int64

	err := Pool.QueryRow(ctx,
		`INSERT INTO devices (imei, sim_number, vehicle_number, chassis_number)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (imei) DO UPDATE 
		 SET sim_number=EXCLUDED.sim_number,
		     vehicle_number=EXCLUDED.vehicle_number,
		     chassis_number=EXCLUDED.chassis_number
		 RETURNING id`,
		d.IMEI, d.SIM, d.VehicleNo, d.ChassisNo,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("local device insert failed: %w", err)
	}
	return id, nil
}

func CreateDevice(d Device) (int64, error) {
	ctx := context.Background()

	// Check if exists
	var id int64
	err := Pool.QueryRow(ctx, `SELECT id FROM devices WHERE imei=$1`, d.IMEI).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("check device failed: %v", err)
	}

	// Try backend
	if backendURL != "" {
		payload := map[string]string{
			"imei":           d.IMEI,
			"sim_number":     d.SIM,
			"vehicle_number": d.VehicleNo,
			"chassis_number": d.ChassisNo,
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", backendURL+"/api/devices/create", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()

			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				var out struct{ ID int64 }
				if json.NewDecoder(resp.Body).Decode(&out) == nil {
					d.ID = out.ID
					id, _ = insertDeviceLocally(ctx, d)
					return id, nil
				}
			}

			resBody, _ := io.ReadAll(resp.Body)
			log.Println("⚠ backend create failed:", string(resBody))
		}
	}

	// Fallback
	return insertDeviceLocally(ctx, d)
}

/* ---------------------- SAVE POSITIONS ---------------------- */

func SavePositions(list []Position) error {
	if len(list) == 0 {
		return nil
	}

	ctx := context.Background()
	deviceCache := make(map[int64]bool)

	for _, p := range list {
		if p.Latitude == 0 || p.Longitude == 0 {
			log.Printf("⚠ Skip device %d: invalid coords (%f,%f)", p.DeviceID, p.Latitude, p.Longitude)
			continue
		}

		exists, ok := deviceCache[p.DeviceID]
		if !ok {
			err := Pool.QueryRow(ctx,
				"SELECT EXISTS(SELECT 1 FROM devices WHERE id=$1)",
				p.DeviceID,
			).Scan(&exists)
			if err != nil {
				log.Printf("⚠ Device lookup failed %d: %v", p.DeviceID, err)
				continue
			}
			deviceCache[p.DeviceID] = exists
		}

		if !exists {
			log.Printf("⚠ Skip: device %d does not exist", p.DeviceID)
			continue
		}

		_, err := Pool.Exec(ctx,
			`INSERT INTO positions 
			 (device_id, lat, lng, speed, angle, altitude, satellites, timestamp, imei)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			p.DeviceID, p.Latitude, p.Longitude, p.Speed,
			p.Angle, p.Altitude, p.Satellites, p.Timestamp, p.IMEI,
		)
		if err != nil {
			log.Printf("❌ Position insert failed device %d: %v", p.DeviceID, err)
			continue
		}

		// Update last location for convenience
		if err := UpdateDeviceLastPosition(p.DeviceID, p.Latitude, p.Longitude); err != nil {
			log.Printf("⚠ last_pos update failed %d: %v", p.DeviceID, err)
		}
	}

	return nil
}

func SavePosition(p Position) error {
	return SavePositions([]Position{p})
}

func SavePositionRaw(deviceID int64, lat, lng, speed float64, angle, altitude, sat int, t string) error {
	if lat == 0 || lng == 0 {
		return errors.New("invalid lat/lng")
	}
	ts, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %v", err)
	}
	return SavePositions([]Position{
		{
			DeviceID:   deviceID,
			Latitude:   lat,
			Longitude:  lng,
			Speed:      speed,
			Angle:      float64(angle),
			Altitude:   float64(altitude),
			Satellites: sat,
			Timestamp:  ts,
		},
	})
}

/* ---------------------- DEVICES LIST ---------------------- */

func GetAllDevicesWithLastPosition() ([]Device, error) {
	ctx := context.Background()
	rows, err := Pool.Query(ctx,
		`SELECT
			d.id,
			d.imei,
			COALESCE(d.sim_number, '')     AS sim_number,
			COALESCE(d.vehicle_number, '') AS vehicle_number,
			COALESCE(d.chassis_number, '') AS chassis_number,
			p.lat AS last_lat,
			p.lng AS last_lng
		FROM devices d
		LEFT JOIN LATERAL (
			SELECT lat, lng
			FROM positions
			WHERE device_id = d.id AND timestamp < now()
			ORDER BY timestamp DESC
			LIMIT 1
		) p ON true
		ORDER BY d.id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]Device, 0, 64)
	for rows.Next() {
		var d Device
		if err := rows.Scan(
			&d.ID,
			&d.IMEI,
			&d.SIM,
			&d.VehicleNo,
			&d.ChassisNo,
			&d.LastLat,
			&d.LastLng,
		); err != nil {
			return nil, fmt.Errorf("scan devices: %w", err)
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows devices: %w", err)
	}

	return devices, nil
}

func GetAllDevicesWithVehicle() ([]DeviceWithVehicle, error) {
	ctx := context.Background()
	rows, err := Pool.Query(ctx,
		`SELECT 
			d.id,
			d.imei,
			COALESCE(d.sim_number, '') AS sim_number,
			v.id, v.vehicle_number, v.chassis_number,
			v.owner_name, v.owner_id_number, v.owner_phone, v.owner_email,
			p.lat AS last_lat, p.lng AS last_lng
		 FROM devices d
		 LEFT JOIN vehicles v ON d.vehicle_id = v.id
		 LEFT JOIN LATERAL (
		       SELECT lat, lng
		         FROM positions
		        WHERE device_id = d.id AND timestamp < now()
		        ORDER BY timestamp DESC
		        LIMIT 1
		 ) p ON true
		 ORDER BY d.id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch devices failed: %v", err)
	}
	defer rows.Close()

	var devices []DeviceWithVehicle
	for rows.Next() {
		var d DeviceWithVehicle
		if err := rows.Scan(
			&d.DeviceID, &d.IMEI, &d.SIM,
			&d.VehicleID, &d.VehicleNumber, &d.ChassisNo,
			&d.OwnerName, &d.OwnerID, &d.OwnerPhone, &d.OwnerEmail,
			&d.LastLat, &d.LastLng,
		); err != nil {
			log.Println("scan error:", err)
			continue
		}
		devices = append(devices, d)
	}
	return devices, nil
}

/* ---------------------- LATEST POSITION ---------------------- */

func GetLatestPositionByIMEI(imei string) (*Position, error) {
	ctx := context.Background()
	row := Pool.QueryRow(ctx,
		`SELECT device_id, imei, lat, lng, speed, angle, altitude, satellites, timestamp
		   FROM positions
		  WHERE imei=$1 AND timestamp < now()
		  ORDER BY timestamp DESC
		  LIMIT 1`, imei,
	)

	var p Position
	if err := row.Scan(
		&p.DeviceID, &p.IMEI, &p.Latitude, &p.Longitude,
		&p.Speed, &p.Angle, &p.Altitude, &p.Satellites, &p.Timestamp,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

/* ---------------------- PING ---------------------- */

func PingDB() error {
	return Pool.Ping(context.Background())
}
