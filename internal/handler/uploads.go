package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	appMiddleware "rentspace/backend/internal/middleware"
	"rentspace/backend/internal/storage"
	"rentspace/backend/internal/store"
)

const maxSlipBytes  = 5 * 1024 * 1024 // 5MB
const maxPhotoBytes = 5 * 1024 * 1024 // 5MB

var allowedImageMIMEs = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/heic": "heic",
	"image/heif": "heif",
}

type UploadsHandler struct {
	q  store.Store
	r2 *storage.R2Client
}

func NewUploadsHandler(q store.Store, r2 *storage.R2Client) *UploadsHandler {
	return &UploadsHandler{q: q, r2: r2}
}

type slipPresignRequest struct {
	BookingID string `json:"booking_id"`
	FileSize  int64  `json:"file_size"`
	MimeType  string `json:"mime_type"`
}

type slipPresignResponse struct {
	UploadURL string `json:"upload_url"`
	FileKey   string `json:"file_key"`
	PublicURL string `json:"public_url"`
}

func (h *UploadsHandler) PresignSlipUpload(w http.ResponseWriter, r *http.Request) {
	if h.r2 == nil {
		Error(w, http.StatusServiceUnavailable, "file storage not configured")
		return
	}

	claims := appMiddleware.ClaimsFromCtx(r.Context())
	if claims.Role != "renter" {
		Error(w, http.StatusForbidden, "only renters can upload slips")
		return
	}

	var body slipPresignRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.FileSize <= 0 || body.FileSize > maxSlipBytes {
		Error(w, http.StatusBadRequest, "file must be between 1 byte and 5MB")
		return
	}

	mime := strings.ToLower(body.MimeType)
	ext, ok := allowedImageMIMEs[mime]
	if !ok {
		Error(w, http.StatusBadRequest, "unsupported file type; use JPEG, PNG, WebP, or HEIC")
		return
	}

	bookingID, err := uuid.Parse(body.BookingID)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid booking ID")
		return
	}

	booking, err := h.q.GetBookingByID(r.Context(), bookingID)
	if err != nil {
		Error(w, http.StatusNotFound, "booking not found")
		return
	}
	if booking.RenterID != claims.ProfileID {
		Error(w, http.StatusForbidden, "this booking does not belong to you")
		return
	}
	if booking.Status != store.BookingStatusAwaitingPayment {
		Error(w, http.StatusUnprocessableEntity, "booking is not awaiting payment")
		return
	}

	key := fmt.Sprintf("slips/%s/%s.%s", bookingID, uuid.New(), ext)
	uploadURL, err := h.r2.PresignUpload(r.Context(), key, mime, body.FileSize, 15*time.Minute)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate upload URL")
		return
	}

	JSON(w, http.StatusOK, slipPresignResponse{
		UploadURL: uploadURL,
		FileKey:   key,
		PublicURL: h.r2.PublicURL(key),
	})
}

type photoPresignRequest struct {
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
}

type deletePhotoRequest struct {
	FileKey string `json:"file_key"`
}

func (h *UploadsHandler) PresignPhotoUpload(w http.ResponseWriter, r *http.Request) {
	if h.r2 == nil {
		Error(w, http.StatusServiceUnavailable, "file storage not configured")
		return
	}

	claims := appMiddleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owners can upload space photos")
		return
	}

	var body photoPresignRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.FileSize <= 0 || body.FileSize > maxPhotoBytes {
		Error(w, http.StatusBadRequest, "file must be between 1 byte and 5MB")
		return
	}

	mime := strings.ToLower(body.MimeType)
	ext, ok := allowedImageMIMEs[mime]
	if !ok {
		Error(w, http.StatusBadRequest, "unsupported file type; use JPEG, PNG, WebP, or HEIC")
		return
	}

	key := fmt.Sprintf("photos/%s/%s.%s", claims.ProfileID, uuid.New(), ext)
	uploadURL, err := h.r2.PresignUpload(r.Context(), key, mime, body.FileSize, 15*time.Minute)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate upload URL")
		return
	}

	JSON(w, http.StatusOK, slipPresignResponse{
		UploadURL: uploadURL,
		FileKey:   key,
		PublicURL: h.r2.PublicURL(key),
	})
}

func (h *UploadsHandler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	if h.r2 == nil {
		Error(w, http.StatusServiceUnavailable, "file storage not configured")
		return
	}

	claims := appMiddleware.ClaimsFromCtx(r.Context())
	if claims.Role != "owner" {
		Error(w, http.StatusForbidden, "only owners can delete space photos")
		return
	}

	var body deletePhotoRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Only allow deleting keys that belong to this owner profile
	expectedPrefix := fmt.Sprintf("photos/%s/", claims.ProfileID)
	if !strings.HasPrefix(body.FileKey, expectedPrefix) {
		Error(w, http.StatusForbidden, "cannot delete this file")
		return
	}

	if err := h.r2.DeleteObject(r.Context(), body.FileKey); err != nil {
		Error(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
