package lang

import (
	"slices"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/diag"
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

func TestRoutesOverUnknownKeyKept(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\nTable authors {\n\tid int64 [pk, increment]\n\temail string\n}\n",
		"app/routes.volt": "package app\n\nimport (\n\tdb\n)\n\n" +
			"Scope /api {\n\tget /authors/:id(int) db.AuthorGet\n\tpatch /authors/:id(int) db.AuthorUpdate\n\tget /writers db.AuthorList\n}\n\n" +
			"Scope /v2 [name: v2] {\n\tresources db.authors [default, only: (index, show)]\n}\n\n" +
			"Scope /admin [name: admin] {\n\tresources db.authors [only: (show, edit)]\n}\n",
	})
	if len(diags) != 1 || !strings.Contains(diags[0].Msg, `unknown type "int64"`) || !diag.HasErrors(diags) {
		t.Fatalf("want the one unknown-type error, which keeps the package from the generator; got %v", diags)
	}
	got := map[string]*RouteInfo{}
	for _, r := range pr.Packages["app"].Routes {
		got[r.Method+" "+r.Pattern] = r
	}
	for _, tc := range []struct {
		route      string
		unresolved bool
		keyed      bool
	}{
		{"GET /api/authors/{id}", true, false},
		{"PATCH /api/authors/{id}", true, false},
		{"GET /api/writers", false, false},
		{"GET /v2/authors", false, false},
		{"GET /v2/authors/{id}", true, true},
		{"GET /admin/authors/{id}", false, true},
		{"GET /admin/authors/{id}/edit", false, true},
	} {
		r := got[tc.route]
		if r == nil {
			t.Errorf("%s was dropped; a route over a key of unknown type is kept and checked", tc.route)
			continue
		}
		if r.Query != nil && r.Query.Unresolved != tc.unresolved {
			t.Errorf("%s: Query.Unresolved = %v, want %v", tc.route, r.Query.Unresolved, tc.unresolved)
		}
		if r.Query == nil && r.Controller != "Authors" {
			t.Errorf("%s: neither a query nor the Authors controller: %+v", tc.route, r)
		}
		if tc.keyed && (len(r.Params) != 1 || r.Params[0].Type != tUnresolved) {
			t.Errorf("%s: Params %+v; want the one key, its type unresolved", tc.route, r.Params)
		}
	}
}
