package listener

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"time"
)

// StartGT06Listener starts a TCP server for GT06 devices
func StartGT06Listener(port int) {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start GT06 TCP listener: %v", err)
	}
	defer ln.Close()

	log.Printf("✅ GT06 TCP listener running on port %d\n", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("GT06 accept error:", err)
			continue
		}
		go handleGT06Connection(conn)
	}
}

func handleGT06Connection(conn net.Conn) {
	defer conn.Close()
	log.Printf("🔌 GT06 device connected: %s", conn.RemoteAddr())

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

	residual := make([]byte, 0)
	tmp := make([]byte, 4096)

	for {
		n, err := conn.Read(tmp)
		if err != nil {
			log.Printf("GT06 connection closed: %v", err)
			return
		}
		if n == 0 {
			continue
		}

		residual = append(residual, tmp[:n]...)
		log.Printf("📦 GT06 RAW (%d bytes): %X", n, tmp[:n])

		// Extract as many full GT06 frames as possible
		for {
			frame, ok := tryExtractGT06Frame(&residual)
			if !ok {
				break
			}

			log.Printf("✅ GT06 FRAME (%d bytes): %X", len(frame), frame)

			// TODO: parse protocol number + decode login/heartbeat/location + reply ACK
			// You can start by logging frame[3] which is often "protocol number"
			// Example:
			// protoNo := frame[3]
			// log.Printf("GT06 protoNo=0x%02X", protoNo)

			// TODO send ACK depending on protoNo + serial number.
		}
	}
}

// tryExtractGT06Frame pulls one complete frame from a stream buffer.
// GT06 common framing: 0x78 0x78 ... 0x0D 0x0A
func tryExtractGT06Frame(residual *[]byte) ([]byte, bool) {
	data := *residual
	if len(data) < 5 {
		return nil, false
	}

	// Find start 0x78 0x78
	start := bytes.Index(data, []byte{0x78, 0x78})
	if start < 0 {
		// discard junk if no header found
		*residual = (*residual)[:0]
		return nil, false
	}
	if start > 0 {
		data = data[start:]
		*residual = data
		if len(data) < 5 {
			return nil, false
		}
	}

	// Need start(2) + len(1)
	if len(data) < 3 {
		return nil, false
	}

	pktLen := int(data[2])
	// Total frame = start(2) + lengthByte(1) + pktLen + stop(2)
	total := 2 + 1 + pktLen + 2
	if len(data) < total {
		return nil, false
	}

	frame := make([]byte, total)
	copy(frame, data[:total])

	// Validate stop bytes
	if frame[total-2] != 0x0D || frame[total-1] != 0x0A {
		// Not aligned: drop one byte and retry
		*residual = data[1:]
		return nil, true
	}

	// consume
	*residual = data[total:]
	return frame, true
}
