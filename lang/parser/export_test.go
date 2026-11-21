package parser

func ChunkTexts(r *Reuse) []string {
	out := make([]string, len(r.chunks))
	for i, c := range r.chunks {
		out[i] = c.text
	}
	return out
}
