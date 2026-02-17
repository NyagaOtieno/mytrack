package listener

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// StartUniGuardListener starts a TCP server for UniGuard devices (ASCII packets ending with '$')
func StartUniGuardListener(port int) {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start UniGuard TCP listener: %v", err)
	}
	defer ln.Close()

	log.Printf("✅ UniGuard TCP listener running on port %d\n", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("UniGuard accept error:", err)
			continue
		}
		go handleUniGuardConnection(conn)
	}
}

func handleUniGuardConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("🔌 UniGuard device connected: %s", conn.RemoteAddr())

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

	residual := make([]byte, 0)
	tmp := make([]byte, 4096)

	for {
		n, err := conn.Read(tmp)
		if err != nil {
			log.Printf("UniGuard connection closed: %v", err)
			return
		}
		if n == 0 {
			continue
		}

		residual = append(residual, tmp[:n]...)
		log.Printf("📦 UniGuard RAW (%d bytes): %q", n, string(tmp[:n]))

		// UniGuard messages typically end with '$' — extract full messages
		for {
			msg, ok := tryExtractDollarTerminated(&residual)
			if !ok {
				break
			}

			raw := strings.TrimSpace(msg)
			log.Printf("✅ UNIGUARD MSG: %q", raw)

			// Basic split example: S168#IMEI#serial#length#...$
			parts := strings.Split(raw, "#")
			if len(parts) >= 2 {
				imei := strings.TrimSpace(parts[1])
				log.Printf("🔢 UniGuard IMEI: %s", imei)
			}

			// TODO: implement ACK replies (LOCA/SYNC/etc) once you confirm exact packet types from logs.
		}
	}
}

func tryExtractDollarTerminated(residual *[]byte) (string, bool) {
	data := *residual
	if len(data) == 0 {
		return "", false
	}

	idx := bytes.IndexByte(data, '$')
	if idx < 0 {
		return "", false
	}

	msgBytes := make([]byte, idx+1)
	copy(msgBytes, data[:idx+1])

	// consume
	*residual = data[idx+1:]
	return string(msgBytes), true
}
