package itest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Piechutowski/volt/internal/dbtest"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

const benchRows = 10000

func benchDB(b *testing.B) run.DB {
	b.Helper()
	ddl, err := os.ReadFile("volt_schema.sql")
	if err != nil {
		b.Fatal(err)
	}
	return dbtest.Open(b, string(ddl))
}

func benchUsers(round int) []User {
	now := time.Now().UTC()
	us := make([]User, benchRows)
	for i := range us {
		us[i] = User{Email: fmt.Sprintf("u%d-%d@example.com", round, i), Name: "Ann", CreatedAt: now}
	}
	return us
}

// BenchmarkTransactionInsert10k is ten thousand UserCreate steps in one
// rt.Tx (spec §generated-query-surface, the runner): the insert is
// prepared once on the transaction's connection.
func BenchmarkTransactionInsert10k(b *testing.B) {
	ctx := context.Background()
	db := benchDB(b)
	round := 0
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		round++
		us := benchUsers(round)
		b.StartTimer()
		steps := make([]rt.AnyEffect, len(us))
		for i, u := range us {
			steps[i] = UserCreate(UserNew().From(u))
		}
		if _, err := run.Run(ctx, db, rt.Tx(steps...)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCreateMany10k is ten thousand rows through UserCreateMany
// (spec §changesets): statements of as many rows as SQLite's parameters
// hold, in one transaction.
func BenchmarkCreateMany10k(b *testing.B) {
	ctx := context.Background()
	db := benchDB(b)
	round := 0
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		round++
		us := benchUsers(round)
		b.StartTimer()
		cs := make([]UserChangeset, len(us))
		for i, u := range us {
			cs[i] = UserNew().From(u)
		}
		if _, err := run.Run(ctx, db, UserCreateMany(cs)); err != nil {
			b.Fatal(err)
		}
	}
}
