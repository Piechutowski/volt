package server

import (
	"net/http"

	"github.com/Piechutowski/volt/compare/chisqlc/api"
)

func (s *server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.q.GetStats(r.Context(), memberFrom(r).ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.Stats{Drafts: st.Drafts, Published: st.Published, CommentsReceived: st.CommentsReceived})
}
