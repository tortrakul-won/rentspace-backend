package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"rentspace/backend/internal/document"
	"rentspace/backend/internal/middleware"
	"rentspace/backend/internal/store"
)

type DocumentsHandler struct {
	q            store.Store
	gotenbergURL string
}

func NewDocumentsHandler(q store.Store, gotenbergURL string) *DocumentsHandler {
	return &DocumentsHandler{q: q, gotenbergURL: gotenbergURL}
}

// GetDocument serves a booking document as PDF.
// Accessible by the booking's renter, the space owner, or an admin.
func (h *DocumentsHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	docTypeSlug := chi.URLParam(r, "docType")
	docType := slugToDocType(docTypeSlug)
	if docType == "" {
		Error(w, http.StatusNotFound, "unknown document type")
		return
	}

	claims := middleware.ClaimsFromCtx(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	booking, err := h.q.GetBookingByID(r.Context(), bookingID)
	if err != nil {
		Error(w, http.StatusNotFound, "booking not found")
		return
	}

	// Only confirmed (or completed) bookings have documents
	if booking.Status != store.BookingStatusConfirmed && booking.Status != store.BookingStatusCompleted {
		Error(w, http.StatusUnprocessableEntity, "documents are only available for confirmed bookings")
		return
	}

	space, err := h.q.GetSpaceByID(r.Context(), booking.SpaceID)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	// Auth: renter of this booking, owner of the space, or admin
	if !claims.IsAdmin {
		profileID := claims.ProfileID
		switch claims.Role {
		case "renter":
			if booking.RenterID != profileID {
				Error(w, http.StatusForbidden, "forbidden")
				return
			}
			// Renter can only access RA and receipt/tax invoice
			if docType != document.DocTypeRentalAgreement &&
				docType != document.DocTypeReceipt &&
				docType != document.DocTypeTaxInvoice {
				Error(w, http.StatusForbidden, "renter cannot access this document type")
				return
			}
		case "owner":
			if space.OwnerID != profileID {
				Error(w, http.StatusForbidden, "forbidden")
				return
			}
		default:
			Error(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	renter, err := h.q.GetProfileByID(r.Context(), booking.RenterID)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	owner, err := h.q.GetProfileByID(r.Context(), space.OwnerID)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	platform, err := document.LoadPlatformConfig(r.Context(), h.q)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	// Resolve docType: receipt slug → receipt or tax_invoice based on owner VAT status
	if docType == document.DocTypeReceipt && owner.IsVatRegistered {
		docType = document.DocTypeTaxInvoice
	}

	pdfBytes, err := document.GenerateDocumentPDF(
		r.Context(),
		h.q,
		h.gotenbergURL,
		booking, space, renter, owner, platform,
		docType,
	)
	if err != nil {
		if errors.Is(err, document.ErrNotMinted) {
			Error(w, http.StatusNotFound, "document not yet generated for this booking")
			return
		}
		ServerError(w, r, err)
		return
	}

	filename := docTypeSlug + "-" + booking.RefCode + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes) //nolint:errcheck
}

func slugToDocType(slug string) string {
	switch slug {
	case "rental-agreement":
		return document.DocTypeRentalAgreement
	case "receipt":
		return document.DocTypeReceipt
	case "payout-statement":
		return document.DocTypePayoutStatement
	case "wht-certificate":
		return document.DocTypeWHTCertificate
	}
	return ""
}

// GetBookingDocumentList returns a list of available document types for a booking.
func (h *DocumentsHandler) GetBookingDocumentList(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	claims := middleware.ClaimsFromCtx(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	booking, err := h.q.GetBookingByID(r.Context(), bookingID)
	if err != nil {
		Error(w, http.StatusNotFound, "booking not found")
		return
	}

	docs, err := h.q.ListBookingDocuments(r.Context(), bookingID)
	if err != nil {
		ServerError(w, r, err)
		return
	}

	type docItem struct {
		DocType   string `json:"doc_type"`
		DocNumber string `json:"doc_number"`
		IssuedAt  string `json:"issued_at"`
		URL       string `json:"url"`
	}

	slugMap := map[string]string{
		document.DocTypeRentalAgreement: "rental-agreement",
		document.DocTypeReceipt:         "receipt",
		document.DocTypeTaxInvoice:      "receipt",
		document.DocTypePayoutStatement: "payout-statement",
		document.DocTypeWHTCertificate:  "wht-certificate",
	}

	items := make([]docItem, 0, len(docs))
	for _, d := range docs {
		slug := slugMap[d.DocType]
		if slug == "" {
			continue
		}
		items = append(items, docItem{
			DocType:   d.DocType,
			DocNumber: d.DocNumber,
			IssuedAt:  d.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
			URL:       "/api/v1/bookings/" + booking.ID.String() + "/documents/" + slug,
		})
	}

	JSON(w, http.StatusOK, items)
}
