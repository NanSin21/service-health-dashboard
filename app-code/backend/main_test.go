package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCheckService(t *testing.T) {
	tests := []struct {
		name       string
		transport  roundTripperFunc
		timeout    time.Duration
		wantStatus string
		wantDetail string
	}{
		{
			name: "2xx response is healthy",
			transport: func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				return mockResponse(http.StatusNoContent, r), nil
			},
			wantStatus: "UP",
		},
		{
			name: "non-2xx response is unhealthy",
			transport: func(r *http.Request) (*http.Response, error) {
				return mockResponse(http.StatusServiceUnavailable, r), nil
			},
			wantStatus: "DOWN",
			wantDetail: "HTTP 503",
		},
		{
			name: "request timeout is reported",
			transport: func(r *http.Request) (*http.Response, error) {
				<-r.Context().Done()
				return nil, r.Context().Err()
			},
			timeout:    10 * time.Millisecond,
			wantStatus: "DOWN",
			wantDetail: "Timeout",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			timeout := test.timeout
			if timeout == 0 {
				timeout = checkTimeout
			}
			client := &http.Client{Transport: test.transport}
			result := checkServiceWithTimeout(client, Service{ID: 7, Name: "test-service", URL: "http://health.test/health"}, timeout)

			if result.Status != test.wantStatus {
				t.Errorf("status = %q, want %q", result.Status, test.wantStatus)
			}
			if result.Details != test.wantDetail {
				t.Errorf("details = %q, want %q", result.Details, test.wantDetail)
			}
			if result.ServiceID != 7 {
				t.Errorf("service ID = %d, want 7", result.ServiceID)
			}
			if result.ResponseTimeMs < 0 {
				t.Errorf("response time = %d, want non-negative", result.ResponseTimeMs)
			}
		})
	}

	t.Run("invalid URL is reported", func(t *testing.T) {
		result := checkService(http.DefaultClient, Service{ID: 8, Name: "bad-url", URL: "://invalid"})
		if result.Status != "DOWN" || result.Details != "Invalid request configuration" {
			t.Fatalf("result = %#v, want DOWN with invalid configuration detail", result)
		}
	})

	t.Run("connection error is reported", func(t *testing.T) {
		result := checkService(http.DefaultClient, Service{ID: 9, Name: "offline", URL: "http://127.0.0.1:0/health"})
		if result.Status != "DOWN" || result.Details != "Connection or request error" {
			t.Fatalf("result = %#v, want DOWN with connection error detail", result)
		}
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func mockResponse(status int, request *http.Request) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    request,
	}
}

type fakeHealthStore struct {
	services       []Service
	listErr        error
	recordErr      error
	mu             sync.Mutex
	recordedChecks []ServiceResult
}

func (store *fakeHealthStore) listServices(context.Context) ([]Service, error) {
	return store.services, store.listErr
}

func (store *fakeHealthStore) recordHealthCheck(_ context.Context, result ServiceResult) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.recordedChecks = append(store.recordedChecks, result)
	return store.recordErr
}

func TestHealthCheckHandler(t *testing.T) {
	t.Run("rejects non-GET requests", func(t *testing.T) {
		store := &fakeHealthStore{}
		recorder := httptest.NewRecorder()
		healthCheckHandler(store, http.DefaultClient)(recorder, httptest.NewRequest(http.MethodPost, "/api/health", nil))

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("reports service lookup errors", func(t *testing.T) {
		store := &fakeHealthStore{listErr: errors.New("database unavailable")}
		recorder := httptest.NewRecorder()
		healthCheckHandler(store, http.DefaultClient)(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
	})

	t.Run("checks services and records results", func(t *testing.T) {
		client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			if strings.HasSuffix(r.URL.Path, "/down") {
				return mockResponse(http.StatusBadGateway, r), nil
			}
			return mockResponse(http.StatusOK, r), nil
		})}

		store := &fakeHealthStore{services: []Service{
			{ID: 1, Name: "healthy", URL: "http://health.test/up"},
			{ID: 2, Name: "unhealthy", URL: "http://health.test/down"},
		}}
		recorder := httptest.NewRecorder()
		healthCheckHandler(store, client)(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var response HealthResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.CheckedAt.IsZero() {
			t.Error("checkedAt should be set")
		}
		if len(response.Services) != 2 {
			t.Fatalf("service count = %d, want 2", len(response.Services))
		}
		if response.Services[0].Status != "UP" || response.Services[1].Status != "DOWN" || response.Services[1].Details != "HTTP 502" {
			t.Errorf("services = %#v, want UP and DOWN (HTTP 502)", response.Services)
		}
		if strings.Contains(recorder.Body.String(), "responseTime") {
			t.Error("API response should not expose response time")
		}

		store.mu.Lock()
		defer store.mu.Unlock()
		if len(store.recordedChecks) != 2 {
			t.Errorf("recorded check count = %d, want 2", len(store.recordedChecks))
		}
	})

	t.Run("reports persistence failures in service details", func(t *testing.T) {
		client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return mockResponse(http.StatusOK, r), nil
		})}

		store := &fakeHealthStore{
			services:  []Service{{ID: 1, Name: "test-service", URL: "http://health.test"}},
			recordErr: errors.New("database write failed"),
		}
		recorder := httptest.NewRecorder()
		healthCheckHandler(store, client)(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

		var response HealthResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(response.Services) != 1 || response.Services[0].Status != "DOWN" || response.Services[0].Details != "Unable to save health-check result" {
			t.Fatalf("services = %#v, want DOWN with persistence failure detail", response.Services)
		}
	})

	t.Run("empty service list returns an empty array", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		healthCheckHandler(&fakeHealthStore{}, http.DefaultClient)(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

		var response HealthResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !reflect.DeepEqual(response.Services, []ServiceResult{}) {
			t.Fatalf("services = %#v, want empty array", response.Services)
		}
	})
}

func TestPostgresHealthStore(t *testing.T) {
	t.Run("loads enabled service rows", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("create SQL mock: %v", err)
		}
		defer db.Close()

		query := `SELECT id, name, url
		FROM services
		WHERE enabled = TRUE
		ORDER BY id`
		mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(
			sqlmock.NewRows([]string{"id", "name", "url"}).
				AddRow(int64(1), "user-service", "http://user/health").
				AddRow(int64(2), "order-service", "http://order/health"),
		)

		services, err := (postgresHealthStore{db: db}).listServices(context.Background())
		if err != nil {
			t.Fatalf("list services: %v", err)
		}
		if len(services) != 2 || services[0] != (Service{ID: 1, Name: "user-service", URL: "http://user/health"}) {
			t.Fatalf("services = %#v, want ordered configured services", services)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("returns query failures", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("create SQL mock: %v", err)
		}
		defer db.Close()

		query := `SELECT id, name, url
		FROM services
		WHERE enabled = TRUE
		ORDER BY id`
		mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(errors.New("query failed"))

		if _, err := (postgresHealthStore{db: db}).listServices(context.Background()); err == nil {
			t.Fatal("list services should return the database error")
		}
	})

	t.Run("records a health check with null detail for success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("create SQL mock: %v", err)
		}
		defer db.Close()

		query := `INSERT INTO health_checks (service_id, status, details, response_time_ms)
		VALUES ($1, $2, NULLIF($3, ''), $4)`
		mock.ExpectExec(regexp.QuoteMeta(query)).
			WithArgs(int64(3), "UP", "", int64(42)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		result := ServiceResult{ServiceID: 3, Name: "payment-service", Status: "UP", ResponseTimeMs: 42}
		if err := (postgresHealthStore{db: db}).recordHealthCheck(context.Background(), result); err != nil {
			t.Fatalf("record health check: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("wraps health-check persistence errors", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("create SQL mock: %v", err)
		}
		defer db.Close()

		query := `INSERT INTO health_checks (service_id, status, details, response_time_ms)
		VALUES ($1, $2, NULLIF($3, ''), $4)`
		mock.ExpectExec(regexp.QuoteMeta(query)).WillReturnError(errors.New("write failed"))

		err = (postgresHealthStore{db: db}).recordHealthCheck(context.Background(), ServiceResult{Name: "user-service"})
		if err == nil || !strings.Contains(err.Error(), "record health check for user-service") {
			t.Fatalf("error = %v, want wrapped persistence error", err)
		}
	})
}
