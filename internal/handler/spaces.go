package handler

import (
	"encoding/json"
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
	claims := middleware.ClaimsFromCtx(r.Context())

	var spaces []store.Space
	var total int64
	var err error

	if claims != nil {
		// Authenticated: exclude all spaces owned by any profile of this user.
		if category != "" {
			cat := store.SpaceCategory(category)
			spaces, err = h.q.ListSpacesByCategoryPaginatedExcludeUser(r.Context(), store.ListSpacesByCategoryPaginatedExcludeUserParams{
				Category: cat,
				UserID:   claims.UserID,
				Lim:      limit,
				Off:      offset,
			})
			if err != nil {
				ServerError(w, r, err)
				return
			}
			total, err = h.q.CountSpacesByCategoryExcludeUser(r.Context(), store.CountSpacesByCategoryExcludeUserParams{
				Category: cat,
				UserID:   claims.UserID,
			})
		} else {
			spaces, err = h.q.ListSpacesPaginatedExcludeUser(r.Context(), store.ListSpacesPaginatedExcludeUserParams{
				UserID: claims.UserID,
				Limit:  limit,
				Offset: offset,
			})
			if err != nil {
				ServerError(w, r, err)
				return
			}
			total, err = h.q.CountSpacesExcludeUser(r.Context(), claims.UserID)
		}
	} else {
		// Unauthenticated: show all active spaces.
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
		MinMinutes:          body.MinMinutes,
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
		MinMinutes:          body.MinMinutes,
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

func (h *SpacesHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can reactivate spaces")
		return
	}

	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	space, err := h.q.SetSpaceActive(r.Context(), store.SetSpaceActiveParams{
		ID:       id,
		IsActive: true,
		OwnerID:  claims.ProfileID,
	})
	if err != nil {
		Error(w, http.StatusNotFound, "space not found or not owned by you")
		return
	}
	JSON(w, http.StatusOK, space)
}

func (h *SpacesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owner profiles can delete spaces")
		return
	}

	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.q.DeleteSpace(r.Context(), store.DeleteSpaceParams{
		ID:      id,
		OwnerID: claims.ProfileID,
	}); err != nil {
		Error(w, http.StatusNotFound, "space not found or not owned by you")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetAvailability returns availability data for a space.
// Without ?from=&to= params → returns weekly open-hours schedule (existing behaviour).
// With    ?from=YYYY-MM-DD&to=YYYY-MM-DD → returns merged day-by-day view including blocks and bookings.
func (h *SpacesHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	// Merged view when date range is provided
	if fromStr != "" && toStr != "" {
		h.getMergedAvailability(w, r, id, fromStr, toStr)
		return
	}

	// Weekly schedule (backward-compatible)
	slots, err := h.q.GetSpaceAvailability(r.Context(), id)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	resp := make([]AvailabilitySlot, 0, len(slots))
	for _, s := range slots {
		resp = append(resp, AvailabilitySlot{
			DayOfWeek: int(s.DayOfWeek),
			OpenTime:  s.OpenTime,
			CloseTime: s.CloseTime,
		})
	}
	JSON(w, http.StatusOK, resp)
}

func (h *SpacesHandler) getMergedAvailability(w http.ResponseWriter, r *http.Request, spaceID interface{ String() string }, fromStr, toStr string) {
	// parse as uuid.UUID
	id, _ := parseUUID(chi.URLParam(r, "id"))

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid from date, use YYYY-MM-DD")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid to date, use YYYY-MM-DD")
		return
	}
	if to.Before(from) {
		Error(w, http.StatusBadRequest, "to must be on or after from")
		return
	}
	// max 90 days range
	if to.Sub(from).Hours()/24 > 90 {
		Error(w, http.StatusBadRequest, "date range cannot exceed 90 days")
		return
	}

	// Fetch weekly schedule
	slots, err := h.q.GetSpaceAvailability(r.Context(), id)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	scheduleByDow := map[int16]*store.SpaceAvailability{}
	for i := range slots {
		scheduleByDow[slots[i].DayOfWeek] = &slots[i]
	}

	// Fetch space blocks in range
	rangeEnd := to.AddDate(0, 0, 1)
	blocks, err := h.q.GetSpaceBlocksInRange(r.Context(), store.GetSpaceBlocksInRangeParams{
		SpaceID:   id,
		StartTime: from,
		EndTime:   rangeEnd,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}

	// Fetch active bookings in range
	bookings, err := h.q.ListActiveBookingsInRange(r.Context(), store.ListActiveBookingsInRangeParams{
		SpaceID:   id,
		StartTime: from,
		EndTime:   rangeEnd,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}

	result := map[string]DayAvailability{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		dow := int16(d.Weekday())
		slot, open := scheduleByDow[dow]

		day := DayAvailability{
			Open:          open,
			BlockedRanges: []BlockedRange{},
		}
		if open {
			day.OpenTime = slot.OpenTime
			day.CloseTime = slot.CloseTime
		}

		// Add space blocks for this day
		for _, b := range blocks {
			if b.StartTime.Format("2006-01-02") == dateKey || overlapsDay(b.StartTime, b.EndTime, d) {
				day.BlockedRanges = append(day.BlockedRanges, BlockedRange{
					From: b.StartTime.Format("15:04"),
					To:   b.EndTime.Format("15:04"),
					Type: "block",
				})
			}
		}

		// Add bookings for this day
		for _, bk := range bookings {
			if bk.StartTime.Format("2006-01-02") == dateKey || overlapsDay(bk.StartTime, bk.EndTime, d) {
				day.BlockedRanges = append(day.BlockedRanges, BlockedRange{
					From: bk.StartTime.Format("15:04"),
					To:   bk.EndTime.Format("15:04"),
					Type: "booking",
				})
			}
		}

		result[dateKey] = day
	}

	JSON(w, http.StatusOK, result)
}

func overlapsDay(start, end time.Time, day time.Time) bool {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	return start.Before(dayEnd) && end.After(dayStart)
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
			if _, err := q.UpsertSpaceAvailability(r.Context(), store.UpsertSpaceAvailabilityParams{
				SpaceID:   id,
				DayOfWeek: int16(slot.DayOfWeek),
				OpenTime:  slot.OpenTime,
				CloseTime: slot.CloseTime,
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
			OpenTime:  s.OpenTime,
			CloseTime: s.CloseTime,
		})
	}
	JSON(w, http.StatusOK, resp)
}
