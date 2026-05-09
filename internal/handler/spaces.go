package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

type SpacesHandler struct {
	q store.Store
}

func NewSpacesHandler(q store.Store) *SpacesHandler {
	return &SpacesHandler{q: q}
}

func (h *SpacesHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)
	offset := (page - 1) * limit
	category := r.URL.Query().Get("category")

	var spaces []store.Space
	var total int64
	var err error

	if category != "" {
		cat := store.SpaceCategory(category)
		spaces, err = h.q.ListSpacesByCategoryPaginated(r.Context(), store.ListSpacesByCategoryPaginatedParams{
			Category: cat,
			Lim:      limit,
			Off:      offset,
		})
		if err != nil {
			ServerError(w, r, err)
			return
		}
		total, err = h.q.CountSpacesByCategory(r.Context(), cat)
	} else {
		spaces, err = h.q.ListSpacesPaginated(r.Context(), store.ListSpacesPaginatedParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			ServerError(w, r, err)
			return
		}
		total, err = h.q.CountSpaces(r.Context())
	}
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, Page[store.Space]{
		Data:    nonNil(spaces),
		Total:   total,
		Page:    page,
		Limit:   limit,
		HasMore: int64(offset)+int64(len(spaces)) < total,
	})
}

// Mine returns all spaces (active and inactive) owned by the authenticated owner profile.
func (h *SpacesHandler) Mine(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can list their spaces")
		return
	}
	page, limit := parsePagination(r)
	offset := (page - 1) * limit

	spaces, err := h.q.ListSpacesByOwnerPaginated(r.Context(), store.ListSpacesByOwnerPaginatedParams{
		OwnerID: claims.ProfileID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	total, err := h.q.CountSpacesByOwner(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, Page[store.Space]{
		Data:    nonNil(spaces),
		Total:   total,
		Page:    page,
		Limit:   limit,
		HasMore: int64(offset)+int64(len(spaces)) < total,
	})
}

func (h *SpacesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	space, err := h.q.GetSpaceByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "space not found")
		return
	}
	JSON(w, http.StatusOK, space)
}

// Create pulls owner_id from the JWT — the request body cannot override it.
func (h *SpacesHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can create spaces")
		return
	}

	var body CreateSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	space, err := h.q.CreateSpace(r.Context(), store.CreateSpaceParams{
		OwnerID:             claims.ProfileID,
		Name:                body.Name,
		Description:         body.Description,
		Location:            body.Location,
		Category:            store.SpaceCategory(body.Category),
		Images:              body.Images,
		HourlyRate:          body.HourlyRate,
		DailyRate:           body.DailyRate,
		MinHours:            body.MinHours,
		Capacity:            body.Capacity,
		Amenities:           body.Amenities,
		WeekendSurchargePct: body.WeekendSurchargePct,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusCreated, space)
}

// Update pulls owner_id from the JWT — the SQL also enforces ownership via WHERE owner_id = $13.
func (h *SpacesHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can update spaces")
		return
	}

	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body CreateSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	space, err := h.q.UpdateSpace(r.Context(), store.UpdateSpaceParams{
		ID:                  id,
		OwnerID:             claims.ProfileID,
		Name:                body.Name,
		Description:         body.Description,
		Location:            body.Location,
		Category:            store.SpaceCategory(body.Category),
		Images:              body.Images,
		HourlyRate:          body.HourlyRate,
		DailyRate:           body.DailyRate,
		MinHours:            body.MinHours,
		Capacity:            body.Capacity,
		Amenities:           body.Amenities,
		WeekendSurchargePct: body.WeekendSurchargePct,
	})
	if err != nil {
		Error(w, http.StatusNotFound, "space not found or not owned by you")
		return
	}
	JSON(w, http.StatusOK, space)
}

// Deactivate pulls owner_id from the JWT to scope the operation to the authenticated owner.
func (h *SpacesHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can deactivate spaces")
		return
	}

	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	space, err := h.q.SetSpaceActive(r.Context(), store.SetSpaceActiveParams{
		ID:       id,
		IsActive: false,
		OwnerID:  claims.ProfileID,
	})
	if err != nil {
		Error(w, http.StatusNotFound, "space not found or not owned by you")
		return
	}
	JSON(w, http.StatusOK, space)
}

// GetAvailability returns the weekly schedule for a space.
func (h *SpacesHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	slots, err := h.q.GetSpaceAvailability(r.Context(), id)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	resp := make([]AvailabilitySlot, 0, len(slots))
	for _, s := range slots {
		resp = append(resp, AvailabilitySlot{
			DayOfWeek: int(s.DayOfWeek),
			OpenTime:  s.OpenTime.Format("15:04"),
			CloseTime: s.CloseTime.Format("15:04"),
		})
	}
	JSON(w, http.StatusOK, resp)
}

// SetAvailability replaces the entire weekly schedule for a space (owner only).
func (h *SpacesHandler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can set availability")
		return
	}

	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body SetAvailabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err = h.q.ExecTx(r.Context(), func(q store.Querier) error {
		if err := q.DeleteSpaceAvailability(r.Context(), id); err != nil {
			return err
		}
		for _, slot := range body.Schedule {
			open, parseErr := time.Parse("15:04", slot.OpenTime)
			if parseErr != nil {
				return fmt.Errorf("invalid open_time %q: %w", slot.OpenTime, parseErr)
			}
			close, parseErr := time.Parse("15:04", slot.CloseTime)
			if parseErr != nil {
				return fmt.Errorf("invalid close_time %q: %w", slot.CloseTime, parseErr)
			}
			if _, err := q.UpsertSpaceAvailability(r.Context(), store.UpsertSpaceAvailabilityParams{
				SpaceID:   id,
				DayOfWeek: int16(slot.DayOfWeek),
				OpenTime:  open,
				CloseTime: close,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}

	slots, err := h.q.GetSpaceAvailability(r.Context(), id)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	resp := make([]AvailabilitySlot, 0, len(slots))
	for _, s := range slots {
		resp = append(resp, AvailabilitySlot{
			DayOfWeek: int(s.DayOfWeek),
			OpenTime:  s.OpenTime.Format("15:04"),
			CloseTime: s.CloseTime.Format("15:04"),
		})
	}
	JSON(w, http.StatusOK, resp)
}
