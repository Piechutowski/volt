// spec: §V4.4.1 — prefixes concatenate, pipes append, names prefix
package app

Pipeline a { use volt.RequestID }
Pipeline b { use volt.Logger }

Scope /api [pipe: a, name: api] {
	Scope /v1 [pipe: b, name: v1] {
		get /health Health.Check
	}
}
