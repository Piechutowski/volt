package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/nao/naming"
)

const localSuffixesMax = 64

func localsAllocate(reserved, bases []string) []string {
	taken := make(map[string]bool, len(reserved)+len(bases))
	for _, name := range reserved {
		taken[name] = true
	}
	out := make([]string, len(bases))
	for i, base := range bases {
		want := "volt" + localGoName(base)
		name := want
		for suffix := 2; taken[name] && suffix < localSuffixesMax; suffix++ {
			name = fmt.Sprintf("%s%d", want, suffix)
		}
		taken[name] = true
		out[i] = name
	}
	return out
}

func localGoName(base string) string {
	name, err := naming.GoName(base)
	if err != nil {
		return "Value"
	}
	return name
}

func localPlural(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "x"), strings.HasSuffix(lower, "z"),
		strings.HasSuffix(lower, "ch"), strings.HasSuffix(lower, "sh"):
		return name + "es"
	case len(lower) > 1 && strings.HasSuffix(lower, "y") && !strings.ContainsRune("aeiou", rune(lower[len(lower)-2])):
		return name[:len(name)-1] + "ies"
	}
	return name + "s"
}

type routeLocals map[string]string

func routeLocalsOf(r *lang.RouteInfo) routeLocals {
	var slots, bases []string
	add := func(slot, base string) {
		slots = append(slots, slot)
		bases = append(bases, base)
	}
	for _, p := range r.Params {
		if p.Type != lang.TString {
			add("path:"+p.Name, p.Name)
		}
	}
	for i, sl := range r.ScopeLoads {
		if r.Query == nil {
			add(fmt.Sprintf("scope:%d", i), sl.Name)
		}
	}
	for i, ld := range r.Loads {
		add(fmt.Sprintf("load:%d", i), ld.Name)
	}
	for i, sl := range r.ScopeLoads {
		add(fmt.Sprintf("hoist:scope:%d", i), sl.Func)
	}
	if q := r.Query; q != nil {
		for _, p := range q.Params {
			switch p.Source {
			case lang.FromQuery, lang.FromList:
				add("query:"+p.Name, p.Name)
			case lang.FromBody:
				add("Params", "Params")
				add("Changeset", "Changeset")
				if q.Changeset.Get != "" {
					add("Row", "Row")
					add("hoist:get", q.Changeset.Get)
				}
			}
		}
		add("hoist:query", q.Method)
		if q.Result != "" {
			result := q.Result
			if q.Many {
				result = localPlural(result)
			}
			add("result", result)
		}
		return routeLocalsBuild(slots, bases)
	}
	if b := r.Body; b != nil {
		add("Body", "Body")
		if b.Kind == lang.BodyChangeset {
			add("Params", "Params")
			if b.Query.Changeset.Get != "" {
				add("Row", "Row")
				add("hoist:get", b.Query.Changeset.Get)
			}
		}
	}
	for i, ld := range r.Loads {
		for j, p := range ld.Query.Params {
			if p.Source != lang.FromPath && p.Source != lang.FromLiteral {
				add(fmt.Sprintf("arg:%d:%d", i, j), fmt.Sprintf("Load%dArg%d", i, j))
			}
		}
		add(fmt.Sprintf("hoist:load:%d", i), ld.Query.Method)
	}
	return routeLocalsBuild(slots, bases)
}

func routeLocalsBuild(slots, bases []string) routeLocals {
	names := localsAllocate(nil, bases)
	out := make(routeLocals, len(slots))
	for i, slot := range slots {
		out[slot] = names[i]
	}
	return out
}

var statusNames = map[int]string{
	http.StatusOK:        "http.StatusOK",
	http.StatusCreated:   "http.StatusCreated",
	http.StatusAccepted:  "http.StatusAccepted",
	http.StatusNoContent: "http.StatusNoContent",
}

func statusName(status int) string {
	if name, ok := statusNames[status]; ok {
		return name
	}
	return fmt.Sprint(status)
}

var routeBytes = map[string]int{
	"volt_handlers.go": 50,
	"volt_router.go":   820,
	"volt_paths.go":    70,
	"volt_routes.go":   175,
	ClientFile:         320,
	ClientBesideFile:   320,
}
