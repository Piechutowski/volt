package lang

import (
	"slices"
	"strings"
	"testing"
)

func TestRouteIndexMatchesPairwiseScan(t *testing.T) {
	var shapes []pathShape
	var gen func(prefix []string)
	gen = func(prefix []string) {
		for _, wild := range []bool{false, true} {
			shapes = append(shapes, pathShape{segs: slices.Clone(prefix), wild: wild})
		}
		if len(prefix) == 3 {
			return
		}
		for _, seg := range []string{"a", "b", ""} {
			gen(append(slices.Clone(prefix), seg))
		}
	}
	gen(nil)
	var routes []*RouteInfo
	for _, sh := range shapes {
		for _, method := range []string{"GET", "POST", ""} {
			routes = append(routes, &RouteInfo{Method: method, shape: sh, Spelled: spelledShape(sh)})
		}
	}
	orders := map[string][]*RouteInfo{"declaration": routes}
	reversed := slices.Clone(routes)
	slices.Reverse(reversed)
	orders["reverse"] = reversed
	for _, stride := range []int{7, 11, 13} {
		var order []*RouteInfo
		for i := range routes {
			order = append(order, routes[i*stride%len(routes)])
		}
		orders["stride "+strings.Repeat("i", stride)] = order
	}
	for name, order := range orders {
		ix := &routeIndex{root: &routeNode{}}
		var accepted []*RouteInfo
		refused := 0
		for i, r := range order {
			r.ord = i
			var want *RouteInfo
			for _, prev := range accepted {
				if routesAmbiguous(prev, r) {
					want = prev
					break
				}
			}
			if got := ix.ambiguous(r); got != want {
				t.Fatalf("%s: route %d %s %s: the trie answers %s, the pairwise scan %s", name, i, methodOrAny(r.Method), r.Spelled, routeName(got), routeName(want))
			}
			if want == nil {
				accepted = append(accepted, r)
				ix.add(r)
			} else {
				refused++
			}
		}
		if refused == 0 || len(accepted) == 0 {
			t.Fatalf("%s: %d refused and %d accepted: the universe exercises nothing", name, refused, len(accepted))
		}
	}
}

func spelledShape(sh pathShape) string {
	var b strings.Builder
	for _, seg := range sh.segs {
		b.WriteByte('/')
		if seg == "" {
			b.WriteString(":p")
		} else {
			b.WriteString(seg)
		}
	}
	if sh.wild {
		b.WriteString("/*rest")
	}
	if b.Len() == 0 {
		return "/"
	}
	return b.String()
}

func routeName(r *RouteInfo) string {
	if r == nil {
		return "nothing"
	}
	return methodOrAny(r.Method) + " " + r.Spelled
}
