package models

type CashRegisterStatus string

const (
	CashRegisterStatusActive   CashRegisterStatus = "ACTIVE"
	CashRegisterStatusDisabled CashRegisterStatus = "DISABLED"
)

type CashRegister struct {
	ID        string             `json:"id"`
	LibraryID string             `json:"libraryId"`
	Name      string             `json:"name"`
	Status    CashRegisterStatus `json:"status"`
	Version   int                `json:"version"`
	CreatedBy string             `json:"createdBy"`
	CreatedAt string             `json:"createdAt"`
	UpdatedAt string             `json:"updatedAt"`
}

type CashRegisterInput struct {
	LibraryID string `json:"libraryId,omitempty"`
	Name      string `json:"name"`
	Version   int    `json:"version,omitempty"`
}

type CashRegisterFilter struct {
	Query  string
	Status CashRegisterStatus
}

type PaymentMethod string

const (
	PaymentMethodCash        PaymentMethod = "CASH"
	PaymentMethodMobileMoney PaymentMethod = "MOBILE_MONEY"
	PaymentMethodCard        PaymentMethod = "CARD"
)

type PaymentStatus string

const (
	PaymentStatusRecorded PaymentStatus = "RECORDED"
	PaymentStatusVoided   PaymentStatus = "VOIDED"
)

type Payment struct {
	ID                string        `json:"id"`
	LibraryID         string        `json:"libraryId"`
	SaleID            string        `json:"saleId"`
	CashRegisterID    string        `json:"cashRegisterId"`
	Method            PaymentMethod `json:"method"`
	Amount            float64       `json:"amount"`
	ExternalReference string        `json:"externalReference,omitempty"`
	Notes             string        `json:"notes,omitempty"`
	Status            PaymentStatus `json:"status"`
	Version           int           `json:"version"`
	RecordedBy        string        `json:"recordedBy"`
	VoidedBy          string        `json:"voidedBy,omitempty"`
	CreatedAt         string        `json:"createdAt"`
	UpdatedAt         string        `json:"updatedAt"`
	VoidedAt          string        `json:"voidedAt,omitempty"`
}

type PaymentInput struct {
	CashRegisterID    string        `json:"cashRegisterId"`
	Method            PaymentMethod `json:"method"`
	Amount            float64       `json:"amount"`
	ExternalReference string        `json:"externalReference,omitempty"`
	Notes             string        `json:"notes,omitempty"`
}

type PaymentVoidInput struct {
	Version int    `json:"version"`
	Reason  string `json:"reason"`
}

type PaymentFilter struct {
	Method PaymentMethod
	Status PaymentStatus
}

type SalePaymentBalance struct {
	SaleID          string  `json:"saleId"`
	LibraryID       string  `json:"libraryId"`
	TotalAmount     float64 `json:"totalAmount"`
	PaidAmount      float64 `json:"paidAmount"`
	RemainingAmount float64 `json:"remainingAmount"`
	PaymentStatus   string  `json:"paymentStatus"`
}
