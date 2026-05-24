package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

type NotificationsHandler struct {
	q store.Store
}

func NewNotificationsHandler(q store.Store) *NotificationsHandler {
	return &NotificationsHandler{q: q}
}

func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}
	notifications, err := h.q.ListNotificationsByProfile(r.Context(), store.ListNotificationsByProfileParams{
		ProfileID: claims.ProfileID,
		Limit:     limit,
	})
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, nonNil(notifications))
}

func (h *NotificationsHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	count, err := h.q.CountUnreadNotifications(r.Context(), claims.ProfileID)
	if err != nil {
		ServerError(w, r, err)
		return
	}
	JSON(w, http.StatusOK, map[string]int64{"count": count})
}

func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.q.MarkNotificationRead(r.Context(), store.MarkNotificationReadParams{
		ID:        id,
		ProfileID: claims.ProfileID,
	}); err != nil {
		ServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())
	if err := h.q.MarkAllNotificationsRead(r.Context(), claims.ProfileID); err != nil {
		ServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
