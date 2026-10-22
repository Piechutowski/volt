// spec: §keywords, §routes, §resources, §datasets, §pipelines, route words are lowercase keywords and Scope and Pipeline are capitalized: each wrong spelling is an error naming the right one
// want: write get, not GET
PACKAGE app

PIPELINE api {
	USE volt.RequestID
}

SCOPE / [pipe: api] {
	GET /x X.Y
	Post /y X.Z
}
