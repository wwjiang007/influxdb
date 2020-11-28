package influxdb

import (
	"context"
)

// ErrScraperTargetNotFound is the error msg for a missing scraper target.
const ErrScraperTargetNotFound = "scraper target not found"

// ops for ScraperTarget Store
const (
	OpListTargets   = "ListTargets"
	OpAddTarget     = "AddTarget"
	OpGetTargetByID = "GetTargetByID"
	OpRemoveTarget  = "RemoveTarget"
	OpUpdateTarget  = "UpdateTarget"
)

// ScraperTarget is a target to scrape
type ScraperTarget struct {
	ID            ID          `json:"id,omitempty"`
	Name          string      `json:"name"`
	Type          ScraperType `json:"type"`
	URL           string      `json:"url"`
	OrgID         ID          `json:"orgID,omitempty"`
	BucketID      ID          `json:"bucketID,omitempty"`
	AllowInsecure bool        `json:"allowInsecure,omitempty"`
}

// ScraperTargetStoreService defines the crud service for ScraperTarget.
type ScraperTargetStoreService interface {
	ListTargets(ctx context.Context, filter ScraperTargetFilter) ([]ScraperTarget, error)
	AddTarget(ctx context.Context, t *ScraperTarget, userID ID) error
	GetTargetByID(ctx context.Context, id ID) (*ScraperTarget, error)
	RemoveTarget(ctx context.Context, id ID) error
	UpdateTarget(ctx context.Context, t *ScraperTarget, userID ID) (*ScraperTarget, error)
}

// ScraperTargetFilter represents a set of filter that restrict the returned results.
type ScraperTargetFilter struct {
	IDs   map[ID]bool `json:"ids"`
	Name  *string     `json:"name"`
	OrgID *ID         `json:"orgID"`
	Org   *string     `json:"org"`
}

// ScraperType defines the scraper methods.
type ScraperType string

// Scraper types
const (
	// PrometheusScraperType parses metrics from a prometheus endpoint.
	PrometheusScraperType = "prometheus"
)

// ValidScraperType returns true is the type string is valid
func ValidScraperType(s string) bool {
	switch s {
	case PrometheusScraperType:
		return true
	default:
		return false
	}
}
