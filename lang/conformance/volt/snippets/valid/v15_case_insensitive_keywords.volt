// spec: §V4.2.1 with docs/spec.md §1.4 — keywords are case-insensitive
PACKAGE app

PIPELINE api { USE volt.RequestID }

SCOPE / [pipe: api] {
	GET /x X.Y
}
