package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HistoryPoint matches the old JSON format so the UI doesn't need big changes
type HistoryPoint struct {
	Time  int64   `json:"t"`
	Price float64 `json:"p"`
}

func main() {
	// InfluxDB connection from env vars (with defaults matching the Helm chart)
	influxURL := getEnv("INFLUXDB_URL", "http://influxdb-service:8086")
	influxToken := getEnv("INFLUXDB_TOKEN", "ticker-secret-token")
	influxOrg := getEnv("INFLUXDB_ORG", "ticker")
	influxBucket := getEnv("INFLUXDB_BUCKET", "ticks")

	client := influxdb2.NewClient(influxURL, influxToken)
	defer client.Close()
	queryAPI := client.QueryAPI(influxOrg)

	// CORS middleware — needed because browser fetches from a different port
	corsHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next(w, r)
		}
	}

	// GET /api/history/{symbol}
	http.HandleFunc("/api/history/", corsHandler(func(w http.ResponseWriter, r *http.Request) {
		// Extract symbol from URL: /api/history/BTC-USD → BTC-USD
		symbol := strings.TrimPrefix(r.URL.Path, "/api/history/")
		if symbol == "" {
			http.Error(w, "Missing symbol", http.StatusBadRequest)
			return
		}

		// Query InfluxDB for the last 24h of ticks for this symbol
		query := fmt.Sprintf(`
			from(bucket: "%s")
				|> range(start: -24h)
				|> filter(fn: (r) => r._measurement == "ticks")
				|> filter(fn: (r) => r.symbol == "%s")
				|> filter(fn: (r) => r._field == "price")
				|> sort(columns: ["_time"])
		`, influxBucket, symbol)

		result, err := queryAPI.Query(context.Background(), query)
		if err != nil {
			log.Printf("Query error for %s: %v", symbol, err)
			http.Error(w, "Query failed", http.StatusInternalServerError)
			return
		}

		var points []HistoryPoint
		for result.Next() {
			record := result.Record()
			points = append(points, HistoryPoint{
				Time:  record.Time().UnixMilli(),
				Price: record.Value().(float64),
			})
		}

		if result.Err() != nil {
			log.Printf("Query result error for %s: %v", symbol, result.Err())
			http.Error(w, "Query failed", http.StatusInternalServerError)
			return
		}

		// Return empty array instead of null
		if points == nil {
			points = []HistoryPoint{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(points)
		log.Printf("Returned %d points for %s", len(points), symbol)
	}))

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	port := getEnv("PORT", "8090")
	fmt.Printf("🚀 Ticker API listening on :%s\n", port)
	fmt.Printf("📊 InfluxDB: %s (org: %s, bucket: %s)\n", influxURL, influxOrg, influxBucket)

	// Expose Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
