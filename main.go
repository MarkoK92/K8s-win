package main

import (
	"context"
	"fmt"
	"k8s-ingester/pb"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func main() {
	// 1. Connect to NATS
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal("NATS Error: ", err)
	}
	defer nc.Close()

	// 2. Connect to InfluxDB
	influxURL := os.Getenv("INFLUXDB_URL")
	if influxURL == "" {
		influxURL = "http://localhost:8086"
	}
	influxToken := os.Getenv("INFLUXDB_TOKEN")
	if influxToken == "" {
		influxToken = "ticker-secret-token"
	}
	influxOrg := os.Getenv("INFLUXDB_ORG")
	if influxOrg == "" {
		influxOrg = "ticker"
	}
	influxBucket := os.Getenv("INFLUXDB_BUCKET")
	if influxBucket == "" {
		influxBucket = "ticks"
	}

	client := influxdb2.NewClient(influxURL, influxToken)
	defer client.Close()
	writeAPI := client.WriteAPIBlocking(influxOrg, influxBucket)

	// Verify InfluxDB connection
	ok, err := client.Health(context.Background())
	if err != nil {
		log.Printf("⚠️  InfluxDB not reachable at %s: %v (will retry on each write)", influxURL, err)
	} else {
		log.Printf("✅ InfluxDB connected: %s (status: %s)", influxURL, ok.Status)
	}

	// 3. Listen for UDP on 5005
	addr := net.UDPAddr{Port: 5005, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal("UDP Error: ", err)
	}
	defer conn.Close()

	fmt.Println("🚀 High-Speed Ingester: BINARY MODE + INFLUXDB ACTIVE")
	fmt.Printf("📊 Writing to InfluxDB: %s (org: %s, bucket: %s)\n", influxURL, influxOrg, influxBucket)

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
		now := time.Now()

		// 4. Create the Protobuf Object
		update := &pb.TickerUpdate{
			Symbol:    parts[0],
			Price:     price,
			Volume:    500,
			Timestamp: timestamp,
		}

		// 5. MARSHAL: Convert the object to binary bytes
		binaryData, err := proto.Marshal(update)
		if err != nil {
			log.Println("Encoding error:", err)
			continue
		}

		// 6. Publish binary bytes to NATS
		nc.Publish("market.ticks", binaryData)

		// 7. Write to InfluxDB
		p := influxdb2.NewPoint("ticks",
			map[string]string{"symbol": parts[0]},
			map[string]interface{}{"price": price},
			now,
		)
		if err := writeAPI.WritePoint(context.Background(), p); err != nil {
			log.Printf("InfluxDB write error: %v", err)
		}

		fmt.Printf("Sent: %s -> $%.2f\n", update.Symbol, update.Price)
	}
}
