package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"fmb920-server/internal/api"
	"fmb920-server/internal/parser"
	"fmb920-server/internal/storage"
)

func main() {
	// Use DATABASE_URL environment variable or default to Railway URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres:ekQHUTYbJdMfomHvDZvtYucbUlNtAPSN@shinkansen.proxy.rlwy.net:34025/railway"
	}

	// Initialize database connection
	err := storage.InitDB(dbURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}
	fmt.Println("✅ Connected to PostgreSQL")

	// Start TCP listener for FMB920 devices
	go startTCPServer(":5027")

	// Start REST API
	api.StartServer(":8080")
}

func startTCPServer(address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("❌ Failed to start TCP listener: %v", err)
	}
	log.Printf("🚀 TCP server listening on %s", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("⚠️ Accept error:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		log.Println("⚠️ Read error:", err)
		return
	}

	data := buf[:n]

	// ---------------- Debug logs ----------------
	log.Printf("📡 Received %d bytes from %s", len(data), conn.RemoteAddr())
	log.Printf("Hex: %x", data)
	log.Printf("ASCII: %s", data)

	// Try extract IMEI for debugging
	imei := parser.TryExtractIMEIFromASCII(data)
	log.Printf("🔎 Extracted IMEI: %s", imei)

	// Parse Codec8 positions
	positions, err := parser.ParseCodec8Records(data)
	if err != nil {
		log.Println("❌ Parse error:", err)
		return
	}

	// Save valid positions only
	for _, p := range positions {
		// ---------------- Debug per record ----------------
		log.Printf("⚙️ Parsed position: %+v", p)

		if p.Latitude == 0 || p.Longitude == 0 {
			log.Printf("⚠️ Skipping zero lat/lng position: %+v", p)
			continue
		}

		ts := time.Unix(0, p.Timestamp*int64(time.Millisecond))
		if ts.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
			log.Printf("⚠️ Skipping invalid timestamp: %+v", ts)
			continue
		}

		// Save a single position
		pos := storage.Position{
			IMEI:      imei,
			Timestamp: ts,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			Speed:     p.Speed,
		}

		err := storage.SavePosition(pos)
		if err != nil {
			log.Printf("❌ DB save error: %v, position: %+v", err, pos)
		} else {
			log.Printf("✅ Saved position: %+v", pos)
		}
	}
}

