package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"rentspace/backend/internal/store"
)

type SpacesHandler struct {
	q *store.Queries
}

func NewSpacesHandler(q *store.Queries) *SpacesHandler {
	return &SpacesHandler{q: q}
}

// List godoc
// @Summary     List all active spaces
// @Tags        spaces
// @Produce     json
// @Success     200 {array}  handler.SpaceResponse
// @Failure     500 {object} handler.ErrorResponse
// @Router      /spaces [get]
func (h *SpacesHandler) List(w http.ResponseWriter, r *http.Request) {
	spaces, err := h.q.ListSpaces(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to fetch spaces")
		return
	}
	JSON(w, http.StatusOK, spaces)
}

// Get godoc
// @Summary     Get a space by ID
// @Tags        spaces
// @Produce     json
// @Param       id  path     string  true  "Space UUID"
// @Success     200 {object} handler.SpaceResponse
// @Failure     400 {object} handler.ErrorResponse
// @Failure     404 {object} handler.ErrorResponse
// @Router      /spaces/{id} [get]
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

// Create godoc
// @Summary     Create a new space listing
// @Tags        spaces
// @Accept      json
// @Produce     json
// @Param       body body     handler.CreateSpaceRequest  true  "Space details"
// @Success     201  {object} handler.SpaceResponse
// @Failure     400  {object} handler.ErrorResponse
// @Failure     500  {object} handler.ErrorResponse
// @Router      /spaces [post]
func (h *SpacesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body store.CreateSpaceParams
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	space, err := h.q.CreateSpace(r.Context(), body)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to create space")
		return
	}
	JSON(w, http.StatusCreated, space)
}

// Update godoc
// @Summary     Update a space listing
// @Tags        spaces
// @Accept      json
// @Produce     json
// @Param       id   path     string                      true  "Space UUID"
// @Param       body body     handler.CreateSpaceRequest  true  "Updated space details"
// @Success     200  {object} handler.SpaceResponse
// @Failure     400  {object} handler.ErrorResponse
// @Failure     500  {object} handler.ErrorResponse
// @Router      /spaces/{id} [put]
func (h *SpacesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body store.UpdateSpaceParams
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.ID = id
	space, err := h.q.UpdateSpace(r.Context(), body)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to update space")
		return
	}
	JSON(w, http.StatusOK, space)
}

// Deactivate godoc
// @Summary     Deactivate a space listing
// @Tags        spaces
// @Produce     json
// @Param       id  path     string  true  "Space UUID"
// @Success     200 {object} handler.SpaceResponse
// @Failure     400 {object} handler.ErrorResponse
// @Failure     500 {object} handler.ErrorResponse
// @Router      /spaces/{id} [delete]
func (h *SpacesHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	// ownerID will come from auth context once auth middleware is wired
	params := store.SetSpaceActiveParams{ID: id, IsActive: false, OwnerID: uuid.UUID{}}
	space, err := h.q.SetSpaceActive(r.Context(), params)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to deactivate space")
		return
	}
	JSON(w, http.StatusOK, space)
}
