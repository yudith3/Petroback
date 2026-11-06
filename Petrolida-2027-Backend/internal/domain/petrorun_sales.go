package domain

import "time"

// RegistrationEndAt is exclusive. A payment reservation always lasts 24 hours,
// including orders created shortly before registration closes.
type PetroRunSaleRules struct {
	RegistrationStartAt time.Time `json:"registration_start_at"`
	RegistrationEndAt   time.Time `json:"registration_end_at"`
	LateStartAt         time.Time `json:"late_start_at"`
	RacepackAt          time.Time `json:"racepack_at"`
	NormalPriceIDR      int64     `json:"normal_price_idr"`
	EarlyBirdPriceIDR   int64     `json:"early_bird_price_idr"`
	EarlyBirdQuota      int       `json:"early_bird_quota"`
	LatePriceIDR        int64     `json:"late_price_idr"`
}

func (r PetroRunSaleRules) RegistrationStatus(now time.Time) string {
	if now.Before(r.RegistrationStartAt) {
		return "upcoming"
	}
	if !now.Before(r.RegistrationEndAt) {
		return "closed"
	}
	return "open"
}

type PetroRunSaleAvailability struct {
	RegistrationStatus string `json:"registration_status"`
	Phase              string `json:"phase"`
	EarlyBirdRemaining int    `json:"early_bird_remaining"`
}

type PetroRunPriceLine struct {
	Tier         string `json:"tier"`
	Quantity     int    `json:"quantity"`
	UnitPriceIDR int64  `json:"unit_price_idr"`
	SubtotalIDR  int64  `json:"subtotal_idr"`
}
