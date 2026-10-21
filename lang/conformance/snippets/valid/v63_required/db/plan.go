package db

import "fmt"

// Plan is the plan of an account (§types).
type Plan string

const (
	PlanFree Plan = "free"
	PlanPro  Plan = "pro"
)

// PlanKnown refuses a plan outside the constants (§validators).
func PlanKnown(p Plan) error {
	switch p {
	case PlanFree, PlanPro:
		return nil
	}
	return fmt.Errorf("%q is not a plan", string(p))
}
