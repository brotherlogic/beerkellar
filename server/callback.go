package server

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	code := r.URL.Query().Get("code")
	token := r.URL.Query().Get("state")
	s.handleAuthResponse(ctx, code, token)
}
