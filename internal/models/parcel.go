package models

import "time"

type Parcel struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Outcome   string    `json:"outcome,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProcessRequest struct {
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}
