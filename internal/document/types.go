package document

const (
	DocTypeRentalAgreement = "rental_agreement"
	DocTypeReceipt         = "receipt"
	DocTypeTaxInvoice      = "tax_invoice"
	DocTypePayoutStatement = "payout_statement"
	DocTypeWHTCertificate  = "wht_certificate"
)

// PlatformConfig holds company identity fields read from system_config.
type PlatformConfig struct {
	NameTH       string
	NameEN       string
	TaxID        string
	Address      string
	BranchNumber string
}
