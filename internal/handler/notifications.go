package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"rentspace/backend/internal/hub"
	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

type NotificationsHandler struct {
	q   store.Store
	hub *hub.Hub
}

func NewNotificationsHandler(q store.Store, h *hub.Hub) *NotificationsHandler {
	return &NotificationsHandler{q: q, hub: h}
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
	resp := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		resp[i] = notificationToResponse(n)
	}
	JSON(w, http.StatusOK, resp)
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

// Stream opens a persistent SSE connection for the authenticated profile.
// Sends a "notification" event whenever a new notification is created for this profile.
// Sends a heartbeat comment every 30 s to keep the connection alive through proxies.
func (h *NotificationsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromCtx(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, unsub := h.hub.Subscribe(claims.ProfileID)
	defer unsub()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// pushNotification writes a notification to the DB and publishes it to any active SSE connection.
// When params.BookingID is valid, previous notifications for that booking are superseded first —
// callers must not call SupersedeNotificationsByBooking separately.
// Errors are swallowed — notifications are best-effort and must not fail the parent transaction.
func pushNotification(ctx context.Context, q store.Querier, h *hub.Hub, params store.CreateNotificationParams) {
	if params.BookingID.Valid {
		if err := q.SupersedeNotificationsByBooking(ctx, params.BookingID.UUID); err != nil {
			log.Printf("supersede notifications for booking %s: %v", params.BookingID.UUID, err)
		}
	}
	n, err := q.CreateNotification(ctx, params)
	if err == nil && h != nil {
		h.Publish(params.ProfileID, hub.Event{
			Type:    "notification",
			Payload: notificationToResponse(n),
		})
	}
}
