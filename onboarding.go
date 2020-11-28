package influxdb

import (
	"context"
	"time"
)

// OnboardingService represents a service for the first run.
type OnboardingService interface {
	// IsOnboarding determine if onboarding request is allowed.
	IsOnboarding(ctx context.Context) (bool, error)

	// OnboardInitialUser OnboardingResults.
	OnboardInitialUser(ctx context.Context, req *OnboardingRequest) (*OnboardingResults, error)

	// OnboardUser creates a new user/org/buckets
	OnboardUser(ctx context.Context, req *OnboardingRequest) (*OnboardingResults, error)
}

// OnboardingResults is a group of elements required for first run.
type OnboardingResults struct {
	User   *User          `json:"user"`
	Org    *Organization  `json:"org"`
	Bucket *Bucket        `json:"bucket"`
	Auth   *Authorization `json:"auth"`
}

// OnboardingRequest is the request
// to setup defaults.
type OnboardingRequest struct {
	User            string        `json:"username"`
	Password        string        `json:"password"`
	Org             string        `json:"org"`
	Bucket          string        `json:"bucket"`
	RetentionPeriod time.Duration `json:"retentionPeriodHrs,omitempty"`
	Token           string        `json:"token,omitempty"`
}

func (r *OnboardingRequest) Valid() error {
	if r.User == "" {
		return &Error{
			Code: EEmptyValue,
			Msg:  "username is empty",
		}
	}

	if r.Org == "" {
		return &Error{
			Code: EEmptyValue,
			Msg:  "org name is empty",
		}
	}

	if r.Bucket == "" {
		return &Error{
			Code: EEmptyValue,
			Msg:  "bucket name is empty",
		}
	}
	return nil
}
