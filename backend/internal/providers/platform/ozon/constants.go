package ozon

// PlatformID is the TradeMind-internal platform key.
const PlatformID = "ozon"

// Ozon import task statuses (create / update product flow).
const (
	importStatusImported = "imported"
	importStatusFailed   = "failed"
	importStatusSkipped  = "skipped"
)

const (
	maxPollAttempts     = 45
	pollInterval        = 3 // seconds between import-info polls
	maxListingImages    = 10
	maxNameRunes        = 1000
	maxDescriptionRunes = 6000
	maxOfferIDRunes     = 90
)
