// spec: §scopes — prefixes concatenate, pipes append, names prefix
package app

pipeline a { use volt.RequestID }
pipeline b { use volt.Logger }

scope /api [pipe: a, name: api] {
	scope /v1 [pipe: b, name: v1] {
		get /health Health.Check
	}
}
