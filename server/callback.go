package server

import (
	"context"
	"log"
	"net/http"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func (s *Server) HandleCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Getting Untappd Callback")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	code := r.URL.Query().Get("code")
	token := r.URL.Query().Get("state")
	s.untappd.handleAuthResponse(ctx, s.db, code, token)
}

func (s *Server) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Getting Google Callback")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	users, err := s.db.GetUsers(ctx)
	if err != nil {
		log.Printf("Error getting users for google callback: %v", err)
		return
	}

	var user *pb.User
	for _, u := range users {
		if u.GetGoogleAuthState() == state {
			user = u
			break
		}
	}

	if user == nil {
		log.Printf("Error: No user found for state %v", state)
		w.Write([]byte("Error: Invalid or expired state."))
		return
	}

	tok, err := getGoogleOAuthConfig().Exchange(ctx, code)
	if err != nil {
		log.Printf("Error exchanging google code: %v", err)
		return
	}

	user.GoogleAccessToken = tok.AccessToken
	user.GoogleRefreshToken = tok.RefreshToken
	user.GoogleTasksEnabled = true

	if err := s.db.SaveUser(ctx, user); err != nil {
		log.Printf("Error saving user with google token: %v", err)
		return
	}
	
	w.Write([]byte("Google Account Linked Successfully! You can close this window."))
}
