// Package jinius delivers invoices. Jinius (BoC's B2B platform) has no public e-invoicing
// docs yet, so only the mock exists; a real client implements the same interface.
package jinius

import (
	"context"
	"time"
)

type Invoice struct {
	ID          string
	Number      string
	ClientName  string
	ClientEmail string
	AmountCents int64
	DueDate     time.Time
	PayToken    string
}

type Delivery struct {
	Provider       string // "jinius" or "mock"
	ExternalID     string
	PaymentLinkURL string
}

type Deliverer interface {
	Deliver(ctx context.Context, inv Invoice) (Delivery, error)
}

// Mock "sends" the invoice and points the payment link at Kerdos's own pay page.
type Mock struct {
	PublicURL string // the frontend, e.g. http://localhost:3000
}

func (m Mock) Deliver(_ context.Context, inv Invoice) (Delivery, error) {
	return Delivery{Provider: "mock", ExternalID: "mock-" + inv.ID, PaymentLinkURL: m.PublicURL + "/pay/" + inv.PayToken}, nil
}
