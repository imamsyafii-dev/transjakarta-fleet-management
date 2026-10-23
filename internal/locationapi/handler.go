package locationapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

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

func (handler *Handler) Register(router gin.IRoutes) {
	router.GET("/vehicles/:vehicle_id/location", handler.latest)
	router.GET("/vehicles/:vehicle_id/history", handler.history)
}

func (handler *Handler) latest(c *gin.Context) {
	vehicleID, ok := validatedVehicleID(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), handler.queryTimeout)
	defer cancel()

	value, err := handler.reader.Latest(ctx, vehicleID)
	if errors.Is(err, locationstore.ErrNotFound) {
		writeError(c, http.StatusNotFound, "vehicle location not found")
		return
	}
	if err != nil {
		writeReaderError(c, ctx, err)
		return
	}

	writeJSON(c, http.StatusOK, value)
}

func (handler *Handler) history(c *gin.Context) {
	vehicleID, ok := validatedVehicleID(c)
	if !ok {
		return
	}

	start, err := requiredTimestamp(c.QueryArray("start"), "start")
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	end, err := requiredTimestamp(c.QueryArray("end"), "end")
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	if start > end {
		writeError(
			c,
			http.StatusBadRequest,
			"start must not be after end",
		)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), handler.queryTimeout)
	defer cancel()

	values, err := handler.reader.History(ctx, vehicleID, start, end)
	if err != nil {
		writeReaderError(c, ctx, err)
		return
	}

	if values == nil {
		values = make([]location.Location, 0)
	}

	writeJSON(c, http.StatusOK, values)
}

func validatedVehicleID(c *gin.Context) (string, bool) {
	vehicleID := c.Param("vehicle_id")

	if err := location.ValidateVehicleID(vehicleID); err != nil {
		writeError(c, http.StatusBadRequest, "vehicle_id is invalid")
		return "", false
	}

	return vehicleID, true
}

func requiredTimestamp(
	values []string,
	name string,
) (int64, error) {
	if len(values) == 0 {
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
	c *gin.Context,
	ctx context.Context,
	err error,
) {
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(ctx.Err(), context.DeadlineExceeded) {
		writeError(c, http.StatusGatewayTimeout, "location query timed out")
		return
	}

	writeError(c, http.StatusInternalServerError, "internal server error")
}

func writeError(
	c *gin.Context,
	status int,
	message string,
) {
	writeJSON(c, status, errorResponse{Error: message})
}

func writeJSON(
	c *gin.Context,
	status int,
	payload any,
) {
	c.JSON(status, payload)
}
