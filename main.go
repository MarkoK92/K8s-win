package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/nats-io/nats.go"
	// In a real project, you'd import your generated 'ticker' package here
)

func main() {
	// 1. Connect to NATS (Use the K8s service name)
	nc, err := nats.Connect("nats://nats-service:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	// 2. Setup UDP Listener
	addr := net.UDPAddr{Port: 5005, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Println("🚀 Go Ingester: Listening for UDP and Publishing to NATS...")

	buf := make([]byte, 1024)
	for {
		n, _, _ := conn.ReadFromUDP(buf)
		rawData := string(buf[:n])

		// 3. Parse and Encode to Protobuf
		// Simulation: BTC-USD,64500.25
		parts := strings.Split(rawData, ",")
		if len(parts) < 2 {
			continue
		}

		// This is where you'd use the ticker.TickerUpdate struct
		fmt.Printf("Encoding %s: %s\n", parts[0], parts[1])

		// 4. Publish to NATS
		// In Phase 2, we send the raw data; in Phase 3, we send 'protoData'
		nc.Publish("market.ticks", []byte(rawData))
	}
}
