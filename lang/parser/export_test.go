package parser

// ChunkTexts is the text of each chunk of a parse, in order: the
// boundaries the scanner cut (§3.2.5), for tests that edit at them.
func ChunkTexts(r *Reuse) []string {
	out := make([]string, len(r.chunks))
	for i, c := range r.chunks {
		out[i] = c.text
	}
	return out
}
