package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

var testSpaceID = uuid.MustParse("00000000-0000-0000-0001-000000000001")

func stubSpace() store.Space {
	return store.Space{
		ID:                  testSpaceID,
		OwnerID:             testProfileID,
		Name:                "Test Studio",
		Description:         "A nice studio",
		Location:            "Bangkok",
		Category:            store.SpaceCategoryStudio,
		Images:              []string{},
		HourlyRate:          500,
		DailyRate:           3000,
		MinHours:            2,
		Capacity:            10,
		Amenities:           []string{"wifi"},
		WeekendSurchargePct: 20,
		IsActive:            true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func withSpaceID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func ownerCtx(r *http.Request) *http.Request {
	claims := &middleware.Claims{ProfileID: testProfileID, Role: "owner"}
	return r.WithContext(middleware.ContextWithClaims(r.Context(), claims))
}

func renterCtx(r *http.Request) *http.Request {
	claims := &middleware.Claims{ProfileID: testProfileID, Role: "renter"}
	return r.WithContext(middleware.ContextWithClaims(r.Context(), claims))
}

// --- Mine ---

func TestSpacesHandler_Mine_Success(t *testing.T) {
	q := &mockStore{
		listSpacesByOwner: func(_ context.Context, ownerID uuid.UUID) ([]store.Space, error) {
			if ownerID != testProfileID {
				t.Errorf("expected ownerID=%s, got %s", testProfileID, ownerID)
			}
			return []store.Space{stubSpace()}, nil
		},
	}
	h := NewSpacesHandler(q)
	r := ownerCtx(httptest.NewRequest(http.MethodGet, "/spaces/mine", nil))
	w := httptest.NewRecorder()
	h.Mine(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var spaces []store.Space
	decodeJSON(t, w.Body, &spaces)
	if len(spaces) != 1 {
		t.Errorf("expected 1 space, got %d", len(spaces))
	}
}

func TestSpacesHandler_Mine_ForbiddenForRenter(t *testing.T) {
	h := NewSpacesHandler(&mockStore{})
	r := renterCtx(httptest.NewRequest(http.MethodGet, "/spaces/mine", nil))
	w := httptest.NewRecorder()
	h.Mine(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// --- Create ---

func TestSpacesHandler_Create_Success(t *testing.T) {
	q := &mockStore{
		createSpace: func(_ context.Context, arg store.CreateSpaceParams) (store.Space, error) {
			if arg.WeekendSurchargePct != 20 {
				t.Errorf("expected WeekendSurchargePct=20, got %d", arg.WeekendSurchargePct)
			}
			return stubSpace(), nil
		},
	}
	h := NewSpacesHandler(q)

	body, _ := json.Marshal(map[string]any{
		"name": "Test Studio", "description": "A nice studio", "location": "Bangkok",
		"category": "Studio", "images": []string{}, "hourly_rate": 500, "daily_rate": 3000,
		"min_hours": 2, "capacity": 10, "amenities": []string{"wifi"}, "weekend_surcharge_pct": 20,
	})
	r := ownerCtx(httptest.NewRequest(http.MethodPost, "/spaces", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	var resp store.Space
	decodeJSON(t, w.Body, &resp)
	if resp.WeekendSurchargePct != 20 {
		t.Errorf("expected weekend_surcharge_pct=20 in response, got %d", resp.WeekendSurchargePct)
	}
}

func TestSpacesHandler_Create_ForbiddenForRenter(t *testing.T) {
	h := NewSpacesHandler(&mockStore{})
	r := renterCtx(httptest.NewRequest(http.MethodPost, "/spaces", nil))
	w := httptest.NewRecorder()
	h.Create(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// --- GetAvailability ---

func TestSpacesHandler_GetAvailability_Success(t *testing.T) {
	openTime, _ := time.Parse("15:04", "09:00")
	closeTime, _ := time.Parse("15:04", "18:00")

	q := &mockStore{
		getSpaceAvailability: func(_ context.Context, spaceID uuid.UUID) ([]store.SpaceAvailability, error) {
			return []store.SpaceAvailability{
				{ID: uuid.New(), SpaceID: spaceID, DayOfWeek: 1, OpenTime: openTime, CloseTime: closeTime},
			}, nil
		},
	}
	h := NewSpacesHandler(q)

	r := withSpaceID(httptest.NewRequest(http.MethodGet, "/spaces/"+testSpaceID.String()+"/availability", nil), testSpaceID.String())
	w := httptest.NewRecorder()
	h.GetAvailability(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var slots []AvailabilitySlot
	decodeJSON(t, w.Body, &slots)
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if slots[0].OpenTime != "09:00" {
		t.Errorf("expected open_time=09:00, got %s", slots[0].OpenTime)
	}
	if slots[0].CloseTime != "18:00" {
		t.Errorf("expected close_time=18:00, got %s", slots[0].CloseTime)
	}
}

// --- SetAvailability ---

func TestSpacesHandler_SetAvailability_Success(t *testing.T) {
	openTime, _ := time.Parse("15:04", "09:00")
	closeTime, _ := time.Parse("15:04", "18:00")

	upsertCalled := 0
	q := &mockStore{
		deleteSpaceAvailability: func(_ context.Context, _ uuid.UUID) error { return nil },
		upsertSpaceAvailability: func(_ context.Context, arg store.UpsertSpaceAvailabilityParams) (store.SpaceAvailability, error) {
			upsertCalled++
			return store.SpaceAvailability{SpaceID: arg.SpaceID, DayOfWeek: arg.DayOfWeek, OpenTime: arg.OpenTime, CloseTime: arg.CloseTime}, nil
		},
		getSpaceAvailability: func(_ context.Context, spaceID uuid.UUID) ([]store.SpaceAvailability, error) {
			return []store.SpaceAvailability{
				{SpaceID: spaceID, DayOfWeek: 1, OpenTime: openTime, CloseTime: closeTime},
				{SpaceID: spaceID, DayOfWeek: 2, OpenTime: openTime, CloseTime: closeTime},
			}, nil
		},
	}
	h := NewSpacesHandler(q)

	body, _ := json.Marshal(SetAvailabilityRequest{
		Schedule: []AvailabilitySlot{
			{DayOfWeek: 1, OpenTime: "09:00", CloseTime: "18:00"},
			{DayOfWeek: 2, OpenTime: "09:00", CloseTime: "18:00"},
		},
	})
	r := ownerCtx(withSpaceID(
		httptest.NewRequest(http.MethodPut, "/spaces/"+testSpaceID.String()+"/availability", bytes.NewReader(body)),
		testSpaceID.String(),
	))
	w := httptest.NewRecorder()
	h.SetAvailability(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if upsertCalled != 2 {
		t.Errorf("expected 2 upsert calls, got %d", upsertCalled)
	}
}

func TestSpacesHandler_SetAvailability_ForbiddenForRenter(t *testing.T) {
	h := NewSpacesHandler(&mockStore{})
	r := renterCtx(withSpaceID(
		httptest.NewRequest(http.MethodPut, "/spaces/"+testSpaceID.String()+"/availability", nil),
		testSpaceID.String(),
	))
	w := httptest.NewRecorder()
	h.SetAvailability(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestSpacesHandler_SetAvailability_InvalidTime(t *testing.T) {
	q := &mockStore{
		deleteSpaceAvailability: func(_ context.Context, _ uuid.UUID) error { return nil },
		upsertSpaceAvailability: func(_ context.Context, _ store.UpsertSpaceAvailabilityParams) (store.SpaceAvailability, error) {
			return store.SpaceAvailability{}, nil
		},
	}
	h := NewSpacesHandler(q)

	body, _ := json.Marshal(SetAvailabilityRequest{
		Schedule: []AvailabilitySlot{{DayOfWeek: 1, OpenTime: "bad", CloseTime: "18:00"}},
	})
	r := ownerCtx(withSpaceID(
		httptest.NewRequest(http.MethodPut, "/spaces/"+testSpaceID.String()+"/availability", bytes.NewReader(body)),
		testSpaceID.String(),
	))
	w := httptest.NewRecorder()
	h.SetAvailability(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
