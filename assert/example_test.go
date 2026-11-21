package assert_test

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Piechutowski/volt/assert"
)

var (
	errAmountZero   = errors.New("amount must not be zero")
	errInsufficient = errors.New("insufficient funds")
)

type account struct{ balance int }

// withdraw returns errors for what a caller can get wrong, and asserts what
// only a bug could break.
func withdraw(a *account, amount int) (err error) {
	assert.That(a != nil, "account must not be nil")

	if amount <= 0 {
		return errAmountZero
	}
	if amount > a.balance {
		return errInsufficient
	}

	before := a.balance
	defer func() {
		// A closure, so the values are read when withdraw returns.
		assert.That(err == nil, "withdraw must succeed after validation")
		assert.That(a.balance == before-amount, "balance must drop by amount")
		assert.That(a.balance >= 0, "balance must not be negative")
	}()

	a.balance -= amount
	return nil
}

func ExampleThat() {
	a := &account{balance: 100}
	fmt.Println(withdraw(a, 30), a.balance)
	fmt.Println(withdraw(a, 500), a.balance)
	// Output:
	// <nil> 70
	// insufficient funds 70
}

type state int

const (
	idle state = iota
	running
	stopped
)

func ExampleFail() {
	next := func(s state) state {
		switch s {
		case idle:
			return running
		case running:
			return stopped
		case stopped:
			return stopped
		}
		assert.Fail("unknown state %d", s)
		return 0
	}
	fmt.Println(next(idle), next(running))
	// Output: 1 2
}

func ExampleMaybe() {
	queue := []int{}
	assert.Maybe(len(queue) == 0) // an empty queue is normal here
}

func ExampleCrash() {
	mux := http.NewServeMux()
	mux.HandleFunc("/withdraw", func(w http.ResponseWriter, r *http.Request) {
		// A failed assertion anywhere below this handler stops the process.
	})
	_ = http.ListenAndServe("localhost:8080", assert.Crash(mux))
}
