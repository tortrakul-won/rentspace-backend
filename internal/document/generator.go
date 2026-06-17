package document

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"rentspace/backend/internal/store"
)

//go:embed templates/base.html
var baseTpl string

//go:embed templates/rental_agreement.html
var rentalAgreementTpl string

//go:embed templates/receipt.html
var receiptTpl string

//go:embed templates/payout_statement.html
var payoutStatementTpl string

//go:embed templates/wht_certificate.html
var whtCertificateTpl string

// profileView is a template-friendly view of a store.Profile.
type profileView struct {
	LegalNameTH  string
	LegalNameEN  string
	TaxID        string
	BranchNumber string
	Address      string
	Phone        string
}

func toProfileView(p store.Profile) profileView {
	addr := strings.Join([]string{
		p.AddressLine1, p.Subdistrict, p.District, p.Province, p.PostalCode,
	}, " ")
	return profileView{
		LegalNameTH:  p.LegalNameTh,
		LegalNameEN:  p.LegalNameEn,
		TaxID:        p.TaxID.String,
		BranchNumber: p.BranchNumber,
		Address:      strings.TrimSpace(addr),
		Phone:        p.Phone,
	}
}

// GenerateDocumentPDF fetches the stored document number, assembles template data,
// renders HTML, and sends it to Gotenberg. Returns raw PDF bytes.
func GenerateDocumentPDF(
	ctx context.Context,
	q store.Querier,
	gotenbergURL string,
	booking store.Booking,
	space store.Space,
	renter store.Profile,
	owner store.Profile,
	platform PlatformConfig,
	docType string,
) ([]byte, error) {
	docNumber, err := GetDocNumber(ctx, q, booking, docType)
	if err != nil {
		if errors.Is(err, ErrNotMinted) {
			return nil, fmt.Errorf("%w: document numbers not yet generated for this booking", ErrNotMinted)
		}
		return nil, err
	}

	issuedAt, err := getIssuedAt(ctx, q, booking, docType)
	if err != nil {
		issuedAt = time.Now()
	}

	html, err := renderTemplate(booking, space, renter, owner, platform, docType, docNumber, issuedAt)
	if err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}

	return renderPDF(ctx, gotenbergURL, html)
}

func getIssuedAt(ctx context.Context, q store.Querier, booking store.Booking, docType string) (time.Time, error) {
	doc, err := q.GetBookingDocument(ctx, store.GetBookingDocumentParams{
		BookingID: booking.ID,
		DocType:   docType,
	})
	if err != nil {
		return time.Time{}, err
	}
	return doc.IssuedAt, nil
}

func renderTemplate(
	booking store.Booking,
	space store.Space,
	renter, owner store.Profile,
	platform PlatformConfig,
	docType, docNumber string,
	issuedAt time.Time,
) ([]byte, error) {
	// strip the {{template "base"}} directive from content tpls since we inline CSS
	contentTpl := contentTemplate(docType)
	if contentTpl == "" {
		return nil, fmt.Errorf("unknown doc type: %s", docType)
	}

	// Build the full HTML: base header + content body
	fullHTML := buildHTML(contentTpl)

	t, err := template.New("doc").Parse(fullHTML)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	data := buildTemplateData(booking, space, renter, owner, platform, docType, docNumber, issuedAt)

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func contentTemplate(docType string) string {
	switch docType {
	case DocTypeRentalAgreement:
		return rentalAgreementTpl
	case DocTypeReceipt, DocTypeTaxInvoice:
		return receiptTpl
	case DocTypePayoutStatement:
		return payoutStatementTpl
	case DocTypeWHTCertificate:
		return whtCertificateTpl
	}
	return ""
}

// buildHTML strips the {{template "base"}} directive and prepends the base CSS header.
func buildHTML(contentTpl string) string {
	content := strings.Replace(contentTpl, `{{template "base"}}`, "", 1)
	return baseTpl + content
}

type docTemplateData struct {
	DocNumber       string
	IssuedAt        string
	Booking         store.Booking
	Space           store.Space
	Renter          profileView
	Owner           profileView
	Platform        PlatformConfig
	StartTime       string
	EndTime         string
	Headcount       string
	TotalPriceExVAT string
	VATLine         bool
	VATRate         string
	VATAmount       string
	GrandTotal      string
	PlatformFee     string
	WHTLine         bool
	WHTRate         string
	WHTAmount       string
	OwnerPayout     string
	NetPayout       string
	RenterAcceptedAt string
	OwnerAcceptedAt  string
	IsTaxInvoice    bool
}

func buildTemplateData(
	booking store.Booking,
	space store.Space,
	renter, owner store.Profile,
	platform PlatformConfig,
	docType, docNumber string,
	issuedAt time.Time,
) docTemplateData {
	loc := time.FixedZone("ICT", 7*60*60)

	vatRate := int(booking.VatRatePct)
	isTaxInvoice := docType == DocTypeTaxInvoice || (docType == DocTypeReceipt && owner.IsVatRegistered)
	if docType == DocTypeTaxInvoice {
		isTaxInvoice = true
	}

	totalExVAT := int(booking.TotalPrice)
	vatAmount := 0
	if isTaxInvoice && vatRate > 0 {
		vatAmount = totalExVAT * vatRate / 100
	}
	grandTotal := totalExVAT + vatAmount

	platformFee := int(booking.PlatformFee)
	ownerPayout := totalExVAT - platformFee

	whtRate := 3
	whtAmount := 0
	whtLine := renter.IsJuristic && (docType == DocTypePayoutStatement || docType == DocTypeWHTCertificate)
	if whtLine {
		whtAmount = ownerPayout * whtRate / 100
	}
	netPayout := ownerPayout - whtAmount

	renterAccepted := ""
	if booking.RenterAcceptedAt.Valid {
		renterAccepted = thaiDateTime(booking.RenterAcceptedAt.Time.In(loc))
	}
	ownerAccepted := ""
	if booking.OwnerAcceptedAt.Valid {
		ownerAccepted = thaiDateTime(booking.OwnerAcceptedAt.Time.In(loc))
	}

	headcount := ""
	if booking.Headcount.Valid {
		headcount = strconv.Itoa(int(booking.Headcount.Int32))
	}

	return docTemplateData{
		DocNumber:        docNumber,
		IssuedAt:         thaiDate(issuedAt.In(loc)),
		Booking:          booking,
		Space:            space,
		Renter:           toProfileView(renter),
		Owner:            toProfileView(owner),
		Platform:         platform,
		StartTime:        thaiDateTime(booking.StartTime.In(loc)),
		EndTime:          thaiDateTime(booking.EndTime.In(loc)),
		Headcount:        headcount,
		TotalPriceExVAT:  baht(int32(totalExVAT)),
		VATLine:          isTaxInvoice && vatRate > 0,
		VATRate:          strconv.Itoa(vatRate),
		VATAmount:        baht(int32(vatAmount)),
		GrandTotal:       baht(int32(grandTotal)),
		PlatformFee:      baht(int32(platformFee)),
		WHTLine:          whtLine,
		WHTRate:          strconv.Itoa(whtRate),
		WHTAmount:        baht(int32(whtAmount)),
		OwnerPayout:      baht(int32(ownerPayout)),
		NetPayout:        baht(int32(netPayout)),
		RenterAcceptedAt: renterAccepted,
		OwnerAcceptedAt:  ownerAccepted,
		IsTaxInvoice:     isTaxInvoice,
	}
}

// LoadPlatformConfig reads platform identity from system_config.
func LoadPlatformConfig(ctx context.Context, q store.Querier) (PlatformConfig, error) {
	keys := []string{
		"platform_name_th", "platform_name_en",
		"platform_tax_id", "platform_address", "platform_branch_number",
	}
	vals := make(map[string]string, len(keys))
	for _, k := range keys {
		v, err := q.GetSystemConfig(ctx, k)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return PlatformConfig{}, fmt.Errorf("system_config[%s]: %w", k, err)
		}
		vals[k] = v
	}
	return PlatformConfig{
		NameTH:       vals["platform_name_th"],
		NameEN:       vals["platform_name_en"],
		TaxID:        vals["platform_tax_id"],
		Address:      vals["platform_address"],
		BranchNumber: vals["platform_branch_number"],
	}, nil
}
