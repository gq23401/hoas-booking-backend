package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	domainbooking "github.com/yourusername/hoas-booking/internal/domain/booking"
	appbooking "github.com/yourusername/hoas-booking/internal/application/booking"
	"github.com/yourusername/hoas-booking/internal/application/ports"
	"github.com/yourusername/hoas-booking/internal/infrastructure/http/middleware"
)

type BookingHandler struct {
	createLaundry *appbooking.CreateLaundryBookingUseCase
	cancelLaundry *appbooking.CancelLaundryBookingUseCase
	createSauna   *appbooking.CreateSaunaBookingUseCase
	bookings      ports.BookingRepository
}

func NewBookingHandler(createLaundry *appbooking.CreateLaundryBookingUseCase, cancelLaundry *appbooking.CancelLaundryBookingUseCase, createSauna *appbooking.CreateSaunaBookingUseCase, bookings ports.BookingRepository) *BookingHandler {
	return &BookingHandler{createLaundry: createLaundry, cancelLaundry: cancelLaundry, createSauna: createSauna, bookings: bookings}
}

func (h *BookingHandler) RegisterRoutes(e *echo.Echo, authMiddleware echo.MiddlewareFunc) {
	g := e.Group("/api/v1", authMiddleware)
	g.GET("/laundry/machines/:machineId/availability", h.GetMachineAvailability)
	g.POST("/laundry/bookings", h.CreateLaundryBooking)
	g.DELETE("/laundry/bookings/:id", h.CancelLaundryBooking)
	g.GET("/laundry/bookings/my", h.GetMyLaundryBookings)
	g.GET("/sauna/:saunaId/availability", h.GetSaunaAvailability)
	g.POST("/sauna/bookings", h.CreateSaunaBooking)
	g.DELETE("/sauna/bookings/:id", h.CancelSaunaBooking)
}

type LaundryBookingResponse struct {
	ID string `json:"id"`; MachineID string `json:"machine_id"`
	StartsAt string `json:"starts_at"`; EndsAt string `json:"ends_at"`; Status string `json:"status"`
}

type TimeSlotResponse struct {
	StartsAt string `json:"starts_at"`; EndsAt string `json:"ends_at"`; IsAvailable bool `json:"is_available"`
}

func toBookingResponse(b *domainbooking.LaundryBooking) LaundryBookingResponse {
	return LaundryBookingResponse{ID: b.ID.String(), MachineID: b.MachineID.String(), StartsAt: b.StartsAt.Format(time.RFC3339), EndsAt: b.EndsAt.Format(time.RFC3339), Status: string(b.Status)}
}

func (h *BookingHandler) GetMachineAvailability(c echo.Context) error {
	machineID, err := uuid.Parse(c.Param("machineId"))
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid machine id") }
	dateStr := c.QueryParam("date")
	if dateStr == "" { dateStr = time.Now().UTC().Format("2006-01-02") }
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "date must be YYYY-MM-DD") }
	slots, err := h.bookings.GetMachineDayAvailability(c.Request().Context(), machineID, date)
	if err != nil { return echo.NewHTTPError(http.StatusInternalServerError, "could not get availability") }
	resp := make([]TimeSlotResponse, len(slots))
	for i, s := range slots {
		resp[i] = TimeSlotResponse{StartsAt: s.StartsAt.Format(time.RFC3339), EndsAt: s.StartsAt.Add(time.Hour).Format(time.RFC3339), IsAvailable: s.IsAvailable}
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BookingHandler) CreateLaundryBooking(c echo.Context) error {
	type requestBody struct {
		MachineID string `json:"machine_id" validate:"required,uuid"`
		StartsAt  string `json:"starts_at"  validate:"required"`
	}
	var body requestBody
	if err := c.Bind(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid request body") }
	if err := c.Validate(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, err.Error()) }
	machineID, err := uuid.Parse(body.MachineID)
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid machine_id") }
	startsAt, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "starts_at must be RFC3339") }
	out, err := h.createLaundry.Execute(c.Request().Context(), appbooking.CreateLaundryBookingInput{UserID: middleware.GetUserID(c), OrgID: middleware.GetOrgID(c), MachineID: machineID, StartsAt: startsAt.UTC()})
	if err != nil { return h.mapDomainError(err) }
	return c.JSON(http.StatusCreated, toBookingResponse(out.Booking))
}

func (h *BookingHandler) CancelLaundryBooking(c echo.Context) error {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid booking id") }
	type requestBody struct { Reason string `json:"reason"` }
	var body requestBody
	_ = c.Bind(&body)
	if err := h.cancelLaundry.Execute(c.Request().Context(), appbooking.CancelLaundryBookingInput{BookingID: bookingID, RequestingUID: middleware.GetUserID(c), OrgID: middleware.GetOrgID(c), Reason: body.Reason}); err != nil {
		return h.mapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *BookingHandler) GetMyLaundryBookings(c echo.Context) error {
	bookings, err := h.bookings.GetUpcomingUserLaundryBookings(c.Request().Context(), middleware.GetUserID(c))
	if err != nil { return echo.NewHTTPError(http.StatusInternalServerError, "could not get bookings") }
	resp := make([]LaundryBookingResponse, len(bookings))
	for i, b := range bookings { resp[i] = toBookingResponse(b) }
	return c.JSON(http.StatusOK, resp)
}

func (h *BookingHandler) GetSaunaAvailability(c echo.Context) error {
	saunaID, err := uuid.Parse(c.Param("saunaId"))
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid sauna id") }
	dateStr := c.QueryParam("date")
	if dateStr == "" { dateStr = time.Now().UTC().Format("2006-01-02") }
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "date must be YYYY-MM-DD") }
	slots, err := h.bookings.GetSaunaDayAvailability(c.Request().Context(), saunaID, date)
	if err != nil { return echo.NewHTTPError(http.StatusInternalServerError, "could not get availability") }
	resp := make([]TimeSlotResponse, len(slots))
	for i, s := range slots {
		resp[i] = TimeSlotResponse{StartsAt: s.StartsAt.Format(time.RFC3339), EndsAt: s.StartsAt.Add(time.Hour).Format(time.RFC3339), IsAvailable: s.IsAvailable}
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BookingHandler) CreateSaunaBooking(c echo.Context) error {
	type requestBody struct {
		SaunaID  string `json:"sauna_id"  validate:"required,uuid"`
		StartsAt string `json:"starts_at" validate:"required"`
	}
	var body requestBody
	if err := c.Bind(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid request body") }
	if err := c.Validate(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, err.Error()) }
	saunaID, err := uuid.Parse(body.SaunaID)
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid sauna_id") }
	startsAt, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "starts_at must be RFC3339") }
	out, err := h.createSauna.Execute(c.Request().Context(), appbooking.CreateSaunaBookingInput{UserID: middleware.GetUserID(c), OrgID: middleware.GetOrgID(c), SaunaID: saunaID, StartsAt: startsAt.UTC()})
	if err != nil { return h.mapDomainError(err) }
	return c.JSON(http.StatusCreated, map[string]string{"id": out.Booking.ID.String(), "sauna_id": out.Booking.SaunaID.String(), "starts_at": out.Booking.StartsAt.Format(time.RFC3339), "status": string(out.Booking.Status)})
}

func (h *BookingHandler) CancelSaunaBooking(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *BookingHandler) mapDomainError(err error) error {
	switch {
	case errors.Is(err, domainbooking.ErrAccessDenied): return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case errors.Is(err, domainbooking.ErrMachineNotAvailable), errors.Is(err, domainbooking.ErrSaunaNotAvailable), errors.Is(err, domainbooking.ErrMachineUnderMaint): return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domainbooking.ErrQuotaExceeded), errors.Is(err, domainbooking.ErrSaunaQuotaExceeded): return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domainbooking.ErrSlotInPast), errors.Is(err, domainbooking.ErrSlotTooFarAhead), errors.Is(err, domainbooking.ErrSlotMustBeOnHour), errors.Is(err, domainbooking.ErrLaundryOutsideHours), errors.Is(err, domainbooking.ErrSaunaOutsideHours), errors.Is(err, domainbooking.ErrSaunaNotEligibleDay): return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, domainbooking.ErrBookingNotFound): return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domainbooking.ErrCannotCancelPast): return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default: return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
}
