// analyzers: capturedwrite
package app

import (
	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/nao/rt"
)

type Stats struct{}

func (Stats) Tally(r volt.Request) volt.Response {
	total := 0
	step := func(sql string) rt.Effect[int64] {
		return rt.Then(rt.Lookup[int]{SQL: sql}, func(n int) rt.Effect[int64] {
			total += n //WANT capturedwrite
			return rt.Pure(int64(total))
		})
	}
	return volt.Then(rt.All(step("SELECT 1"), step("SELECT 2")), func([]any) volt.Response { return volt.Render(total) })
}

func (Stats) Seen(r volt.Request) volt.Response {
	seen := map[int]bool{}
	mark := func(ids []int) []int {
		for _, id := range ids {
			seen[id] = true //WANT capturedwrite
		}
		return ids
	}
	return volt.Then(rt.Map[[]int, []int](rt.Query[int]{SQL: "SELECT id FROM posts"}, mark), func(ids []int) volt.Response { return volt.Render(ids) })
}

func (Stats) Squares(r volt.Request) volt.Response {
	calls := 0
	out := rt.ParallelMap([]int{1, 2, 3}, func(n int) int {
		calls++ //WANT capturedwrite
		return n * n
	})
	return volt.Render(out)
}

func (Stats) Sum(r volt.Request) volt.Response {
	return volt.Then(rt.Map(rt.Query[int]{SQL: "SELECT n FROM metrics"}, func(ns []int) int {
		sum := 0
		for _, n := range ns {
			sum += n
		}
		return sum
	}), func(sum int) volt.Response { return volt.Render(sum) })
}

func (Stats) Last(r volt.Request) volt.Response {
	last := 0
	keep := func(n int) { last = n }
	keep(r.Now().Year())
	return volt.Render(last)
}
