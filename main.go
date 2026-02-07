package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s-ingester/pb"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

// HistoryEntry represents a single price point
type HistoryEntry struct {
	Time  int64   `json:"t"`
	Price float64 `json:"p"`
}

var (
	historyDir  = "data"
	historyLock sync.Mutex
	maxHistory  = 200 // Keep last 200 ticks per symbol
)

func appendToHistory(symbol string, price float64, timestamp int64) {
	historyLock.Lock()
	defer historyLock.Unlock()

	// Ensure data directory exists
	os.MkdirAll(historyDir, 0755)

	filePath := filepath.Join(historyDir, symbol+".json")

	// Read existing history
	var history []HistoryEntry
	if data, err := os.ReadFile(filePath); err == nil {
		json.Unmarshal(data, &history)
	}

	// Append new entry
	history = append(history, HistoryEntry{Time: timestamp, Price: price})

	// Trim to max size
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	// Write back
	data, _ := json.Marshal(history)
	os.WriteFile(filePath, data, 0644)
}

func main() {
	// 1. Connect to NATS
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal("NATS Error: ", err)
	}
	defer nc.Close()

	// 2. Listen for UDP on 5005
	addr := net.UDPAddr{Port: 5005, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal("UDP Error: ", err)
	}
	defer conn.Close()

	fmt.Println("🚀 High-Speed Ingester: BINARY MODE + HISTORY ACTIVE")
	fmt.Println("📁 Writing history to ./data/*.json")

	buf := make([]byte, 1024)
	for {
		n, _, _ := conn.ReadFromUDP(buf)
		rawData := strings.TrimSpace(string(buf[:n]))

		// Split input like "BTC-USD,74250.65"
		parts := strings.Split(rawData, ",")
		if len(parts) < 2 {
			continue
		}

		price, _ := strconv.ParseFloat(parts[1], 64)
		timestamp := time.Now().UnixMilli()

		// 3. Create the Protobuf Object
		update := &pb.TickerUpdate{
			Symbol:    parts[0],
			Price:     price,
			Volume:    500,
			Timestamp: timestamp,
		}

		// 4. MARSHAL: Convert the object to binary bytes
		binaryData, err := proto.Marshal(update)
		if err != nil {
			log.Println("Encoding error:", err)
			continue
		}

		// 5. Publish binary bytes to NATS
		nc.Publish("market.ticks", binaryData)

		// 6. Write to history file
		go appendToHistory(parts[0], price, timestamp)

		fmt.Printf("Sent: %s -> $%.2f\n", update.Symbol, update.Price)
	}
}
