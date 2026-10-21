package itest

import "fmt"

// OrderState is a declared column type stored as string, its values the
// constants below (spec §types).
type OrderState string

const (
	OrderStatePending OrderState = "pending"
	OrderStateShipped OrderState = "shipped"
	// OrderStateDelivered is the terminal state.
	OrderStateDelivered OrderState = "delivered"
)

// OrderStateKnown refuses a value outside the constants of OrderState
// (spec §validators).
func OrderStateKnown(v OrderState) error {
	switch v {
	case OrderStatePending, OrderStateShipped, OrderStateDelivered:
		return nil
	}
	return fmt.Errorf("%q is not a known order status", string(v))
}
