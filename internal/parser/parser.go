package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
)

var (
	tcpServerHost   string
	backendTrackURL string
	db              *sql.DB
	httpClient      = &http.Client{Timeout: 10 * time.Second}
	wg              sync.WaitGroup

	positionsHasIoData bool
)

type Position struct {
	IMEI       string                 `json:"imei"`
	Timestamp  time.Time              `json:"timestamp"`
	Latitude   float64                `json:"latitude"`
	Longitude  float64                `json:"longitude"`
	Speed      float64                `json:"speed"`
	Angle      int                    `json:"angle"`
	Altitude   int                    `json:"altitude"`
	Satellites int                    `json:"satellites"`
	IOData     map[string]interface{} `json:"io_data,omitempty"`
	Raw        []byte                 `json:"-"`
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	_ = godotenv.Load()

	tcpServerHost = getEnv("TCP_SERVER_HOST", "0.0.0.0:5027")
	backendTrackURL = getEnv("BACKEND_TRACK_URL", "https://mytrack-production.up.railway.app/api/track")

	pgURL := getEnv("DATABASE_URL", "")
	if pgURL == "" {
		log.Fatal("❌ DATABASE_URL not set")
	}

	var err error
	db, err = sql.Open("postgres", pgURL)
	if err != nil {
		log.Fatalf("❌ Failed to open PostgreSQL connection: %v", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatalf("❌ PostgreSQL ping failed: %v", err)
	}

	positionsHasIoData = checkPositionsHasIoData()
	if positionsHasIoData {
		log.Println("ℹ️ positions.io_data column detected; storing IO JSON")
	} else {
		log.Println("⚠️ positions.io_data column not detected; IO data omitted")
	}
}

func checkPositionsHasIoData() bool {
	var col string
	err := db.QueryRow(`
		SELECT column_name FROM information_schema.columns 
		WHERE table_name='positions' AND column_name='io_data' LIMIT 1
	`).Scan(&col)
	return err == nil && col == "io_data"
}

func main() {
	listener, err := net.Listen("tcp", tcpServerHost)
	if err != nil {
		log.Fatalf("❌ TCP server start failed: %v", err)
	}
	defer listener.Close()

	log.Printf("✅ TCP server listening on %s", tcpServerHost)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("⚠️ Accept error:", err)
			continue
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleConnection(c)
		}(conn)
	}

	wg.Wait()
}

// ---------------- TCP Handling ----------------

func handleConnection(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Printf("🔗 New connection from %s", remote)

	imei, err := readIMEI(conn)
	if err != nil {
		log.Printf("❌ IMEI read failed from %s: %v", remote, err)
		return
	}

	deviceID, err := ensureDevice(imei)
	if err != nil {
		log.Printf("❌ Device lookup failed for IMEI %s: %v", imei, err)
		return
	}

	residual := make([]byte, 0)
	tmp := make([]byte, 4096)

	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		n, err := conn.Read(tmp)
		if err != nil {
			if err != io.EOF {
				log.Printf("🔌 Read error for %s: %v", imei, err)
			}
			return
		}

		residual = append(residual, tmp[:n]...)

		// Parse frames
		for len(residual) >= 12 {
			packetLen := int(binary.BigEndian.Uint32(residual[:4]))
			if packetLen <= 0 || packetLen > 5*1024*1024 {
				residual = residual[4:]
				continue
			}
			if len(residual) < 4+packetLen {
				break
			}

			frame := residual[4 : 4+packetLen]
			positions, err := ParseCodecFrame(frame, imei)
			if err != nil {
				log.Printf("❌ Frame parse error for %s: %v", imei, err)
			} else {
				log.Printf("ℹ️ Parsed %d positions from %s", len(positions), imei)
			}

			if err := storePositionsBatch(deviceID, imei, positions); err != nil {
				log.Printf("❌ DB insert failed for %s: %v", imei, err)
			}

			postPositionsToBackend(positions)
			sendACK(conn, len(positions))

			residual = residual[4+packetLen:]
		}
	}
}

// ---------------- IMEI ----------------

func readIMEI(conn net.Conn) (string, error) {
	buf := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	conn.SetReadDeadline(time.Time{})

	imei, err := ParseIMEIHandshake(buf[:n])
	if err != nil {
		imei = TryExtractIMEIASCII(buf[:n])
	}
	if imei == "" {
		return "", errors.New("IMEI not found in handshake")
	}

	_, _ = conn.Write([]byte{0x01})
	log.Printf("📡 Device IMEI: %s", imei)
	return imei, nil
}

func ParseIMEIHandshake(data []byte) (string, error) {
	if len(data) < 2 {
		return "", errors.New("IMEI handshake too short")
	}
	imeiLen := int(data[1])
	if imeiLen != 15 {
		return "", fmt.Errorf("unexpected IMEI length: %d", imeiLen)
	}
	if len(data) < 2+imeiLen {
		return "", errors.New("truncated IMEI")
	}
	return string(data[2 : 2+imeiLen]), nil
}

func TryExtractIMEIASCII(data []byte) string {
	for i := 0; i < len(data)-15; i++ {
		ok := true
		for j := 0; j < 15; j++ {
			if data[i+j] < '0' || data[i+j] > '9' {
				ok = false
				break
			}
		}
		if ok {
			return string(data[i : i+15])
		}
	}
	return ""
}

// ---------------- Device ----------------

func ensureDevice(imei string) (int, error) {
	var id int
	err := db.QueryRow("SELECT id FROM devices WHERE imei=$1", imei).Scan(&id)
	if err == nil {
		return id, nil
	}
	return 0, fmt.Errorf("device IMEI %s not found in DB", imei)
}

// ---------------- PostgreSQL ----------------

func storePositionsBatch(deviceID int, imei string, recs []Position) error {
	if len(recs) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var stmt *sql.Stmt
	if positionsHasIoData {
		stmt, err = tx.Prepare(`
			INSERT INTO positions 
			(device_id, lat, lng, speed, angle, altitude, satellites, timestamp, imei, io_data)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`)
	} else {
		stmt, err = tx.Prepare(`
			INSERT INTO positions 
			(device_id, lat, lng, speed, angle, altitude, satellites, timestamp, imei)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`)
	}
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range recs {
		ioJSON, _ := json.Marshal(r.IOData)
		if positionsHasIoData {
			_, err = stmt.Exec(deviceID, r.Latitude, r.Longitude, r.Speed,
				r.Angle, r.Altitude, r.Satellites, r.Timestamp.UTC(), imei, ioJSON)
		} else {
			_, err = stmt.Exec(deviceID, r.Latitude, r.Longitude, r.Speed,
				r.Angle, r.Altitude, r.Satellites, r.Timestamp.UTC(), imei)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------- Backend ----------------

func postPositionsToBackend(positions []Position) {
	if len(positions) == 0 {
		return
	}
	payload := make([]map[string]interface{}, 0, len(positions))
	for _, p := range positions {
		payload = append(payload, map[string]interface{}{
			"device_id":  0,
			"imei":       p.IMEI,
			"timestamp":  p.Timestamp.UTC().Format(time.RFC3339),
			"latitude":   p.Latitude,
			"longitude":  p.Longitude,
			"speed":      p.Speed,
			"angle":      p.Angle,
			"altitude":   p.Altitude,
			"satellites": p.Satellites,
			"io_data":    p.IOData,
		})
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", backendTrackURL, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("❌ Backend post failed: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("📬 Backend response (%d): %.200s", resp.StatusCode, string(body))
}

// ---------------- ACK ----------------

func sendACK(conn net.Conn, count int) {
	// Teltonika ACK must be exactly 4 bytes — DO NOT ADD TRAILING 0x01
	ack := make([]byte, 4)
	binary.BigEndian.PutUint32(ack, uint32(count))
	_, _ = conn.Write(ack)
	log.Printf("✔ ACK sent: %d", count)
}


// ---------------- Parser ----------------

func ParseCodecFrame(data []byte, imei string) ([]Position, error) {
	if len(data) < 12 {
		return nil, errors.New("frame too short")
	}

	codec := data[8]
	if codec != 0x08 && codec != 0x8E {
		return nil, fmt.Errorf("unsupported codec %02x", codec)
	}

	recordCount := int(data[9])
	if recordCount == 0 {
		return nil, errors.New("recordCount=0")
	}

	offset := 10
	var positions []Position

	for r := 0; r < recordCount; r++ {
		if offset+15 > len(data) {
			return positions, errors.New("frame truncated during record header")
		}

		// Timestamp (ms)
		tsRaw := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
		offset += 8
		ts := time.Unix(tsRaw/1000, (tsRaw%1000)*1e6)

		// Priority
		offset++

		// GPS
		if offset+15 > len(data) {
			return positions, errors.New("truncated GPS block")
		}

		lonRaw := int32(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
		latRaw := int32(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4

		altitude := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		angle := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		sats := int(data[offset])
		offset++

		speed := float64(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		// IO Extended (Codec 8E)
		ioData := make(map[string]interface{})
		if codec == 0x8E {
			if offset >= len(data) {
				return positions, errors.New("missing IO count")
			}

			totalIO := int(data[offset])
			offset++

			for i := 0; i < totalIO; i++ {
				if offset+2 > len(data) {
					return positions, errors.New("truncated IO element")
				}

				ioID := data[offset]
				offset++

				ioLen := int(data[offset])
				offset++

				if ioLen != 1 && ioLen != 2 && ioLen != 4 && ioLen != 8 {
					return positions, fmt.Errorf("invalid IO length %d", ioLen)
				}

				if offset+ioLen > len(data) {
					return positions, errors.New("IO value overflow")
				}

				ioVal := parseIOValue(data[offset : offset+ioLen])
				offset += ioLen

				ioData[fmt.Sprintf("%d", ioID)] = ioVal
			}
		}

		lat := float64(latRaw) / 1e7
		lon := float64(lonRaw) / 1e7

		if lat != 0 && lon != 0 {
			positions = append(positions, Position{
				IMEI:       imei,
				Timestamp:  ts,
				Latitude:   lat,
				Longitude:  lon,
				Speed:      speed,
				Angle:      angle,
				Altitude:   altitude,
				Satellites: sats,
				IOData:     ioData,
				Raw:        data,
			})
		}
	}

	return positions, nil
}


func parseIOValue(data []byte) interface{} {
	switch len(data) {
	case 1:
		return int(data[0])
	case 2:
		return int(binary.BigEndian.Uint16(data))
	case 4:
		return int(binary.BigEndian.Uint32(data))
	case 8:
		return int64(binary.BigEndian.Uint64(data))
	default:
		return data
	}
}

// ---------------- Utility ----------------

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
