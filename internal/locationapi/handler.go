package locationapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/locationstore"
)

type Reader interface {
	Latest(context.Context, string) (location.Location, error)
	History(context.Context, string, int64, int64) ([]location.Location, error)
}

type Config struct {
	QueryTimeout time.Duration
}

type Handler struct {
	reader       Reader
	queryTimeout time.Duration
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(reader Reader, config Config) (*Handler, error) {
	if reader == nil {
		return nil, errors.New("location API reader must not be nil")
	}

	if config.QueryTimeout <= 0 {
		return nil, errors.New(
			"location API query timeout must be greater than zero",
		)
	}

	return &Handler{
		reader:       reader,
		queryTimeout: config.QueryTimeout,
	}, nil
}

func (handler *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /vehicles/{vehicle_id}/location",
		handler.latest,
	)
	mux.HandleFunc(
		"GET /vehicles/{vehicle_id}/history",
		handler.history,
	)
}

func (handler *Handler) latest(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := validatedVehicleID(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), handler.queryTimeout)
	defer cancel()

	value, err := handler.reader.Latest(ctx, vehicleID)
	if errors.Is(err, locationstore.ErrNotFound) {
		writeError(w, http.StatusNotFound, "vehicle location not found")
		return
	}
	if err != nil {
		writeReaderError(w, ctx, err)
		return
	}

	writeJSON(w, http.StatusOK, value)
}

func (handler *Handler) history(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := validatedVehicleID(w, r)
	if !ok {
		return
	}

	start, err := requiredTimestamp(r.URL.Query(), "start")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	end, err := requiredTimestamp(r.URL.Query(), "end")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if start > end {
		writeError(
			w,
			http.StatusBadRequest,
			"start must not be after end",
		)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), handler.queryTimeout)
	defer cancel()

	values, err := handler.reader.History(ctx, vehicleID, start, end)
	if err != nil {
		writeReaderError(w, ctx, err)
		return
	}

	if values == nil {
		values = make([]location.Location, 0)
	}

	writeJSON(w, http.StatusOK, values)
}

func validatedVehicleID(
	w http.ResponseWriter,
	r *http.Request,
) (string, bool) {
	vehicleID := r.PathValue("vehicle_id")

	if err := location.ValidateVehicleID(vehicleID); err != nil {
		writeError(w, http.StatusBadRequest, "vehicle_id is invalid")
		return "", false
	}

	return vehicleID, true
}

func requiredTimestamp(
	query url.Values,
	name string,
) (int64, error) {
	values, exists := query[name]
	if !exists {
		return 0, fmt.Errorf("%s query parameter is required", name)
	}

	if len(values) != 1 {
		return 0, fmt.Errorf(
			"%s query parameter must appear exactly once",
			name,
		)
	}

	value, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf(
			"%s query parameter must be a positive integer",
			name,
		)
	}

	return value, nil
}

func writeReaderError(
	w http.ResponseWriter,
	ctx context.Context,
	err error,
) {
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(ctx.Err(), context.DeadlineExceeded) {
		writeError(w, http.StatusGatewayTimeout, "location query timed out")
		return
	}

	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	payload any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}
