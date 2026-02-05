package server

import (
	"context"
	"log"
	"net/http"
	"time"
)

func (s *Server) HandleCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Getting Callback")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	code := r.URL.Query().Get("code")
	token := r.URL.Query().Get("state")
	s.handleAuthResponse(ctx, s.untappd, code, token)
}
