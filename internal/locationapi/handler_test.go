package locationapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/locationstore"
)

type fakeReader struct {
	latestFn  func(context.Context, string) (location.Location, error)
	historyFn func(
		context.Context,
		string,
		int64,
		int64,
	) ([]location.Location, error)
}

func (reader fakeReader) Latest(
	ctx context.Context,
	vehicleID string,
) (location.Location, error) {
	if reader.latestFn == nil {
		return location.Location{}, errors.New("unexpected Latest call")
	}

	return reader.latestFn(ctx, vehicleID)
}

func (reader fakeReader) History(
	ctx context.Context,
	vehicleID string,
	start int64,
	end int64,
) ([]location.Location, error) {
	if reader.historyFn == nil {
		return nil, errors.New("unexpected History call")
	}

	return reader.historyFn(ctx, vehicleID, start, end)
}

func sampleLocation(timestamp int64) location.Location {
	return location.Location{
		VehicleID: "B1234XYZ",
		Latitude:  -6.2088,
		Longitude: 106.8456,
		Timestamp: timestamp,
	}
}

func newMux(
	t *testing.T,
	reader Reader,
	timeout time.Duration,
) *gin.Engine {
	t.Helper()

	handler, err := New(reader, Config{
		QueryTimeout: timeout,
	})
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	gin.SetMode(gin.TestMode)

	router := gin.New()
	handler.Register(router)

	return router
}

func decodeError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) errorResponse {
	t.Helper()

	var response errorResponse

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	return response
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	validReader := fakeReader{}

	tests := []struct {
		name    string
		reader  Reader
		timeout time.Duration
	}{
		{
			name:    "nil reader",
			timeout: time.Second,
		},
		{
			name:    "zero timeout",
			reader:  validReader,
			timeout: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, err := New(
				test.reader,
				Config{QueryTimeout: test.timeout},
			)

			if err == nil {
				t.Fatal("expected error")
			}

			if handler != nil {
				t.Fatal("expected nil handler")
			}
		})
	}
}

func TestLatestReturnsLocation(t *testing.T) {
	want := sampleLocation(100)

	mux := newMux(t, fakeReader{
		latestFn: func(
			_ context.Context,
			vehicleID string,
		) (location.Location, error) {
			if vehicleID != want.VehicleID {
				t.Fatalf("vehicle ID = %q, want %q", vehicleID, want.VehicleID)
			}

			return want, nil
		},
	}, time.Second)

	request := httptest.NewRequest(
		http.MethodGet,
		"/vehicles/B1234XYZ/location",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var got location.Location
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got != want {
		t.Fatalf("response = %#v, want %#v", got, want)
	}
}

func TestLatestReturnsNotFound(t *testing.T) {
	mux := newMux(t, fakeReader{
		latestFn: func(
			context.Context,
			string,
		) (location.Location, error) {
			return location.Location{}, locationstore.ErrNotFound
		},
	}, time.Second)

	request := httptest.NewRequest(
		http.MethodGet,
		"/vehicles/B1234XYZ/location",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	response := decodeError(t, recorder)
	if response.Error != "vehicle location not found" {
		t.Fatalf("error = %q", response.Error)
	}
}

func TestLatestRejectsInvalidVehicleID(t *testing.T) {
	mux := newMux(t, fakeReader{
		latestFn: func(
			context.Context,
			string,
		) (location.Location, error) {
			t.Fatal("reader must not be called")
			return location.Location{}, nil
		},
	}, time.Second)

	request := httptest.NewRequest(
		http.MethodGet,
		"/vehicles/B%201234XYZ/location",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLatestHidesInternalError(t *testing.T) {
	mux := newMux(t, fakeReader{
		latestFn: func(
			context.Context,
			string,
		) (location.Location, error) {
			return location.Location{}, errors.New("database connection failed")
		},
	}, time.Second)

	request := httptest.NewRequest(
		http.MethodGet,
		"/vehicles/B1234XYZ/location",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	response := decodeError(t, recorder)
	if response.Error != "internal server error" {
		t.Fatalf("error = %q", response.Error)
	}
}

func TestHistoryReturnsLocations(t *testing.T) {
	want := []location.Location{
		sampleLocation(10),
		sampleLocation(20),
	}

	mux := newMux(t, fakeReader{
		historyFn: func(
			_ context.Context,
			vehicleID string,
			start int64,
			end int64,
		) ([]location.Location, error) {
			if vehicleID != "B1234XYZ" {
				t.Fatalf("vehicle ID = %q", vehicleID)
			}
			if start != 10 || end != 20 {
				t.Fatalf("range = %d-%d, want 10-20", start, end)
			}

			return want, nil
		},
	}, time.Second)

	request := httptest.NewRequest(
		http.MethodGet,
		"/vehicles/B1234XYZ/history?start=10&end=20",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var got []location.Location
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("response length = %d, want %d", len(got), len(want))
	}
}

func TestHistoryReturnsEmptyArray(t *testing.T) {
	mux := newMux(t, fakeReader{
		historyFn: func(
			context.Context,
			string,
			int64,
			int64,
		) ([]location.Location, error) {
			return nil, nil
		},
	}, time.Second)

	request := httptest.NewRequest(
		http.MethodGet,
		"/vehicles/B1234XYZ/history?start=10&end=20",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if body := recorder.Body.String(); body != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}

func TestHistoryRejectsInvalidQuery(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{
			name:   "missing start",
			target: "/vehicles/B1234XYZ/history?end=2",
		},
		{
			name:   "duplicate start",
			target: "/vehicles/B1234XYZ/history?start=1&start=2&end=3",
		},
		{
			name:   "non numeric start",
			target: "/vehicles/B1234XYZ/history?start=one&end=2",
		},
		{
			name:   "zero end",
			target: "/vehicles/B1234XYZ/history?start=1&end=0",
		},
		{
			name:   "start after end",
			target: "/vehicles/B1234XYZ/history?start=3&end=2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := newMux(t, fakeReader{
				historyFn: func(
					context.Context,
					string,
					int64,
					int64,
				) ([]location.Location, error) {
					t.Fatal("reader must not be called")
					return nil, nil
				},
			}, time.Second)

			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			recorder := httptest.NewRecorder()

			mux.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, want %d",
					recorder.Code,
					http.StatusBadRequest,
				)
			}
		})
	}
}

func TestHistoryReturnsTimeout(t *testing.T) {
	mux := newMux(t, fakeReader{
		historyFn: func(
			ctx context.Context,
			_ string,
			_ int64,
			_ int64,
		) ([]location.Location, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}, 5*time.Millisecond)

	request := httptest.NewRequest(
		http.MethodGet,
		"/vehicles/B1234XYZ/history?start=1&end=2",
		nil,
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusGatewayTimeout,
		)
	}
}
