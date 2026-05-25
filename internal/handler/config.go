package handler

import (
	"database/sql"
	"errors"
	"net/http"

	"rentspace/backend/internal/store"
)

type ConfigHandler struct{ q store.Store }

func NewConfigHandler(q store.Store) *ConfigHandler { return &ConfigHandler{q: q} }

func (h *ConfigHandler) PaymentConfig(w http.ResponseWriter, r *http.Request) {
	get := func(key string) string {
		v, err := h.q.GetSystemConfig(r.Context(), key)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ""
		}
		return v
	}

	JSON(w, http.StatusOK, map[string]string{
		"promptpay_number": get("promptpay_number"),
		"promptpay_name":   get("promptpay_name"),
		"promptpay_qr_url": get("promptpay_qr_url"),
	})
}
