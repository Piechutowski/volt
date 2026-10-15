// spec: §routes rule 1 with docs/spec.md §case-sensitivity-and-terminology — keywords are case-insensitive
PACKAGE app

PIPELINE api { USE volt.RequestID }

SCOPE / [pipe: api] {
	GET /x X.Y
}
