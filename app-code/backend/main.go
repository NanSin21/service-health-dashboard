package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const checkTimeout = 5 * time.Second

const defaultDatabaseURL = "postgres://service_dashboard:dev_password@localhost:5433/service_dashboard?sslmode=disable"

//go:embed schema.sql
var schema embed.FS

type Service struct {
	ID   int64
	Name string
	URL  string
}

type ServiceResult struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Details        string `json:"details,omitempty"`
	ServiceID      int64  `json:"-"`
	ResponseTimeMs int64  `json:"-"`
}

type HealthResponse struct {
	CheckedAt time.Time       `json:"checkedAt"`
	Services  []ServiceResult `json:"services"`
}

func connectDatabase(ctx context.Context) (*sql.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	schemaSQL, err := schema.ReadFile("schema.sql")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("read database schema: %w", err)
	}
	for _, statement := range strings.Split(string(schemaSQL), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize database schema: %w", err)
		}
	}

	return db, nil
}

func listServices(ctx context.Context, db *sql.DB) ([]Service, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, url
		FROM services
		WHERE enabled = TRUE
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}
	defer rows.Close()

	services := make([]Service, 0)
	for rows.Next() {
		var service Service
		if err := rows.Scan(&service.ID, &service.Name, &service.URL); err != nil {
			return nil, fmt.Errorf("read service: %w", err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}

	return services, nil
}

func checkService(client *http.Client, service Service) ServiceResult {
	result := ServiceResult{
		Name:      service.Name,
		ServiceID: service.ID,
		Status:    "DOWN",
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, service.URL, nil)
	if err != nil {
		result.Details = "Invalid request configuration"
		result.ResponseTimeMs = time.Since(start).Milliseconds()
		return result
	}

	resp, err := client.Do(req)
	result.ResponseTimeMs = time.Since(start).Milliseconds()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Details = "Timeout"
		} else {
			result.Details = "Connection or request error"
		}
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		result.Status = "UP"
		return result
	}
	result.Details = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return result
}

func recordHealthCheck(ctx context.Context, db *sql.DB, result ServiceResult) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO health_checks (service_id, status, details, response_time_ms)
		VALUES ($1, $2, NULLIF($3, ''), $4)
	`, result.ServiceID, result.Status, result.Details, result.ResponseTimeMs)
	if err != nil {
		return fmt.Errorf("record health check for %s: %w", result.Name, err)
	}
	return nil
}

func healthHandler(db *sql.DB, client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		services, err := listServices(r.Context(), db)
		if err != nil {
			log.Printf("load services: %v", err)
			http.Error(w, "Unable to load configured services", http.StatusInternalServerError)
			return
		}

		results := make([]ServiceResult, len(services))
		var wg sync.WaitGroup
		for i, service := range services {
			wg.Add(1)
			go func(i int, service Service) {
				defer wg.Done()
				result := checkService(client, service)
				if err := recordHealthCheck(r.Context(), db, result); err != nil {
					log.Printf("%v", err)
					result.Status = "DOWN"
					result.Details = "Unable to save health-check result"
				}
				results[i] = result
			}(i, service)
		}
		wg.Wait()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(HealthResponse{
			CheckedAt: time.Now().UTC(),
			Services:  results,
		}); err != nil {
			log.Printf("encode health response: %v", err)
		}
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := connectDatabase(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	client := &http.Client{}
	http.HandleFunc("/api/health", healthHandler(db, client))

	log.Println("Health API listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
