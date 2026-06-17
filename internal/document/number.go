package document

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"rentspace/backend/internal/store"
)

// mintNumber allocates a new document number and persists it.
// For RA and PS: derived from ref_code, no sequence needed.
// For Receipt, TaxInvoice, WHT: sequential per year.
func mintNumber(ctx context.Context, q store.Querier, booking store.Booking, docType string) (string, error) {
	var docNumber string

	switch docType {
	case DocTypeRentalAgreement:
		docNumber = fmt.Sprintf("RS-RA-%s", booking.RefCode)
	case DocTypePayoutStatement:
		docNumber = fmt.Sprintf("RS-PS-%s", booking.RefCode)
	case DocTypeReceipt, DocTypeTaxInvoice, DocTypeWHTCertificate:
		var prefix string
		switch docType {
		case DocTypeReceipt:
			prefix = "RS-RCP"
		case DocTypeTaxInvoice:
			prefix = "RS-TAX"
		case DocTypeWHTCertificate:
			prefix = "RS-WHT"
		}
		year := booking.CreatedAt.Year()
		seq, err := q.NextDocumentSequence(ctx, store.NextDocumentSequenceParams{
			DocType: docType,
			Year:    int32(year),
		})
		if err != nil {
			return "", fmt.Errorf("next sequence [%s]: %w", docType, err)
		}
		docNumber = fmt.Sprintf("%s-%d-%05d", prefix, year, seq)
	default:
		return "", fmt.Errorf("unknown doc type: %s", docType)
	}

	_, err := q.CreateBookingDocument(ctx, store.CreateBookingDocumentParams{
		BookingID: booking.ID,
		DocType:   docType,
		DocNumber: docNumber,
	})
	if err != nil {
		return "", fmt.Errorf("create booking document: %w", err)
	}
	return docNumber, nil
}

// GetDocNumber returns the stored document number, or ErrNotMinted if not yet created.
func GetDocNumber(ctx context.Context, q store.Querier, booking store.Booking, docType string) (string, error) {
	doc, err := q.GetBookingDocument(ctx, store.GetBookingDocumentParams{
		BookingID: booking.ID,
		DocType:   docType,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotMinted
		}
		return "", err
	}
	return doc.DocNumber, nil
}

// ErrNotMinted is returned when a document number has not been created yet.
var ErrNotMinted = errors.New("document number not yet minted")

// MintDocumentNumbers creates booking_document rows for all applicable doc types.
// Must be called inside a transaction at the confirmed transition.
func MintDocumentNumbers(ctx context.Context, q store.Querier, booking store.Booking, renter store.Profile, owner store.Profile) error {
	// Rental Agreement — always
	if _, err := mintNumber(ctx, q, booking, DocTypeRentalAgreement); err != nil {
		return err
	}

	// Receipt or TaxInvoice — based on owner VAT registration
	receiptType := DocTypeReceipt
	if owner.IsVatRegistered {
		receiptType = DocTypeTaxInvoice
	}
	if _, err := mintNumber(ctx, q, booking, receiptType); err != nil {
		return err
	}

	// Payout Statement — always
	if _, err := mintNumber(ctx, q, booking, DocTypePayoutStatement); err != nil {
		return err
	}

	// WHT Certificate — only when renter is juristic entity
	if renter.IsJuristic {
		if _, err := mintNumber(ctx, q, booking, DocTypeWHTCertificate); err != nil {
			return err
		}
	}

	return nil
}
