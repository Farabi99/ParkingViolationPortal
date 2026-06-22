package types

import "time"

type ViolationCreatedEvent struct {
	ViolationID  string    `json:"violation_id"`
	LicensePlate string    `json:"license_plate"`
	Type         string    `json:"type"`
	Timestamp    time.Time `json:"timestamp"`
}

type FineCalculatedEvent struct {
	ViolationID       string  `json:"violation_id"`
	FineAmount        float64 `json:"fine_amount"`
	AppliedRuleSetVer int     `json:"applied_rule_set_version"`
}

type PaymentProcessedEvent struct {
	ViolationID   string  `json:"violation_id"`
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"` // "PAID" or "FAILED"
}
