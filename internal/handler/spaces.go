package handler

import (
	"encoding/json"
	"net/http"

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
	spaces, err := h.q.ListSpaces(r.Context())
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, spaces)
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
		OwnerID:     claims.ProfileID,
		Name:        body.Name,
		Description: body.Description,
		Location:    body.Location,
		Category:    store.SpaceCategory(body.Category),
		Images:      body.Images,
		HourlyRate:  body.HourlyRate,
		DailyRate:   body.DailyRate,
		MinHours:    body.MinHours,
		Capacity:    body.Capacity,
		Amenities:   body.Amenities,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusCreated, space)
}

// Update pulls owner_id from the JWT — the SQL also enforces ownership via WHERE owner_id = $12.
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
		ID:          id,
		OwnerID:     claims.ProfileID,
		Name:        body.Name,
		Description: body.Description,
		Location:    body.Location,
		Category:    store.SpaceCategory(body.Category),
		Images:      body.Images,
		HourlyRate:  body.HourlyRate,
		DailyRate:   body.DailyRate,
		MinHours:    body.MinHours,
		Capacity:    body.Capacity,
		Amenities:   body.Amenities,
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
