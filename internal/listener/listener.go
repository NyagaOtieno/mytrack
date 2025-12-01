package listener

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"fmb920-server/internal/parser"
	"fmb920-server/internal/storage"
)

// StartTCPListener starts a TCP server for Teltonika FMB920 devices
func StartTCPListener(port int) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start TCP listener: %v", err)
	}
	defer listener.Close()
	log.Printf("✅ TCP listener running on port %d\n", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Connection accept error:", err)
			continue
		}
		go handleConnection(conn, "tcp")
	}
}

// StartUDPListener starts a UDP server for Teltonika FMB920 devices
func StartUDPListener(port int) {
	addr := fmt.Sprintf(":%d", port)
	udpAddr, _ := net.ResolveUDPAddr("udp", addr)
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to start UDP listener: %v", err)
	}
	defer conn.Close()
	log.Printf("✅ UDP listener running on port %d\n", port)

	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("UDP read error:", err)
			continue
		}

		packet := buf[:n]
		go handleUDP(packet, conn, remoteAddr)
	}
}

// handleConnection handles a TCP connection
func handleConnection(conn net.Conn, proto string) {
	defer conn.Close()
	log.Printf("🔌 Device connected (%s): %s", proto, conn.RemoteAddr())

	residual := make([]byte, 0)
	tmp := make([]byte, 4096)

	for {
		n, err := conn.Read(tmp)
		if err != nil {
			log.Printf("Connection closed: %v", err)
			return
		}
		if n == 0 {
			continue
		}

		residual = append(residual, tmp[:n]...)
		log.Printf("📦 RAW (%d bytes): %X", n, tmp[:n])

		// Handle IMEI login
		if imei := parser.TryExtractIMEIFromASCII(residual); imei != "" && residual[0] == 0x00 {
			log.Printf("🔢 Device login detected. IMEI: %s", imei)
			conn.Write([]byte{0x01}) // login ACK
			residual = residual[len(residual):] // clear
			continue
		}

		// Parse Codec8 frames
		for len(residual) >= 5 {
			if residual[4] != 0x08 {
				residual = residual[1:]
				continue
			}

			records, err := parser.ParseCodec8Records(residual[4:])
			if err != nil {
				log.Println("Codec8 parse error:", err)
				residual = residual[5:]
				continue
			}

			imei := parser.TryExtractIMEIFromASCII(residual)
			if imei == "" {
				imei = "UNKNOWN"
			}

			var dbPositions []storage.Position
			for _, p := range records {
				timestamp := time.UnixMilli(p.Timestamp)
				dbPositions = append(dbPositions, storage.Position{
					IMEI:      imei,
					Timestamp: timestamp,
					Latitude:  p.Latitude,
					Longitude: p.Longitude,
					Speed:     p.Speed,
				})
			}

			if len(dbPositions) > 0 {
				if err := storage.SavePositions(dbPositions); err != nil {
					log.Println("Failed to save positions:", err)
				} else {
					log.Printf("✅ Saved %d position(s) for IMEI %s", len(dbPositions), imei)
				}
			}

			// Send ACK
			sendACK(conn, len(dbPositions))
			residual = residual[4+len(residual[4:]):] // move pointer
		}
	}
}

// handleUDP handles individual UDP packets
func handleUDP(packet []byte, conn *net.UDPConn, addr *net.UDPAddr) {
	log.Printf("📦 RAW UDP (%d bytes) from %s: %X\n", len(packet), addr, packet)

	// IMEI login
	if imei := parser.TryExtractIMEIFromASCII(packet); imei != "" && packet[0] == 0x00 {
		log.Printf("🔢 Device login detected (UDP). IMEI: %s", imei)
		conn.WriteToUDP([]byte{0x01}, addr)
		return
	}

	if len(packet) < 5 || packet[4] != 0x08 {
		log.Printf("❗ Unsupported UDP packet type")
		return
	}

	records, err := parser.ParseCodec8Records(packet[4:])
	if err != nil {
		log.Println("Codec8 UDP parse error:", err)
		return
	}

	imei := parser.TryExtractIMEIFromASCII(packet)
	if imei == "" {
		imei = "UNKNOWN"
	}

	var dbPositions []storage.Position
	for _, p := range records {
		timestamp := time.UnixMilli(p.Timestamp)
		dbPositions = append(dbPositions, storage.Position{
			IMEI:      imei,
			Timestamp: timestamp,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			Speed:     p.Speed,
		})
	}

	if len(dbPositions) > 0 {
		if err := storage.SavePositions(dbPositions); err != nil {
			log.Println("Failed to save UDP positions:", err)
		} else {
			log.Printf("✅ Saved %d UDP position(s) for IMEI %s", len(dbPositions), imei)
		}
	}

	// Send ACK
	ack := make([]byte, 5)
	ack[0] = 0x01
	binary.BigEndian.PutUint32(ack[1:], uint32(len(dbPositions)))
	conn.WriteToUDP(ack, addr)
}

// sendACK sends Codec8 ACK over TCP
func sendACK(conn net.Conn, count int) {
	ack := make([]byte, 5)
	ack[0] = 0x01
	binary.BigEndian.PutUint32(ack[1:], uint32(count))
	conn.Write(ack)
}
