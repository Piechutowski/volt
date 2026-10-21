package db

import (
	"fmt"

	"github.com/Piechutowski/volt/nao/rt"
)

// OrderState is the state of an order (§types).
type OrderState string

const (
	OrderStatePending OrderState = "pending"
	// OrderStatePaid is settled by the processor.
	OrderStatePaid    OrderState = "paid"
	OrderStateShipped OrderState = "shipped"
)

// OrderStateKnown refuses a state outside the constants (§validators).
func OrderStateKnown(s OrderState) error {
	switch s {
	case OrderStatePending, OrderStatePaid, OrderStateShipped:
		return nil
	}
	return fmt.Errorf("%q is not an order state", string(s))
}

// OrderStateKnownNull is OrderStateKnown for a nullable column, which
// NULL passes (§validators).
func OrderStateKnownNull(s rt.Null[OrderState]) error {
	if !s.Valid {
		return nil
	}
	return OrderStateKnown(s.V)
}

// Priority orders the work queue (§types).
type Priority int
