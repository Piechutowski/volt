package db

// State is declared, but the schema's Types are not (spec/types).
type State string

func init() {
	type OrderState string
	_ = OrderState("")
}
