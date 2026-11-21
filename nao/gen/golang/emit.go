package golang

import (
	"strings"

	"github.com/Piechutowski/volt/gen/layout"
)

type (
	elem  = layout.Elem
	elems = []layout.Elem
)

var text = layout.Text

func call(fn string, args ...elem) elem { return layout.Call(fn, args) }

func texts(ss ...string) elems { return layout.Texts(ss) }

func literal(typ string, keys []string, values ...elem) elem {
	return layout.Fields(typ+"{", keys, values, "}")
}

func values(typ string, items ...elem) elem { return layout.List(typ+"{", items, "}") }

func stmt(before string, e elem) elem { return layout.Prefixed(before, e) }

func funcEmit(b *strings.Builder, head string, params []string, results string, body ...elem) {
	sig := layout.Signature(head, params, layout.Results(results), " {")
	if len(body) == 1 {
		sigShape, bodyShape := sig.Shape(layout.At{}), body[0].Shape(layout.At{Indent: 1})
		if !sigShape.Multi && !bodyShape.Multi && sigShape.First+bodyShape.First+len("  }") <= layout.LineColumnsMax {
			sig.Write(b, layout.At{})
			b.WriteByte(' ')
			body[0].Write(b, layout.At{Indent: 1})
			b.WriteString(" }\n\n")
			return
		}
	}
	layout.WriteLine(b, 0, "", sig, "\n")
	for _, s := range body {
		layout.WriteLine(b, 1, "", s, "\n")
	}
	b.WriteString("}\n\n")
}
func calls(fn string, args ...string) elem { return layout.CallStrings(fn, args) }

const gofmtOneLineFieldMax = 30

func funcLit(params []string, results string, body elem) elem {
	return layout.Func(params, results, body)
}
