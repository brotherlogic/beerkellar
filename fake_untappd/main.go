package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	pb "github.com/brotherlogic/beerkellar/proto"
)

const (
	code              = "MADEUP"
	authorizeResponse = `{
  "meta": {
    "http_code": 200
  },
  "response": {
    "access_token": "MADEUPTOKEN"
  }
}`
)

var (
	port = flag.Int("port", 8085, "Server port for fake traffic")
)

type Server struct {
	checkins []*pb.Checkin
}

// This handles the initial request
func (s *Server) HandleOauth1(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling")
	redirectUrl := r.URL.Query().Get("redirect_url")
	state := r.URL.Query().Get("state")

	// Given those we just immediatly hit the callback URL
	url := fmt.Sprintf("%v?code=%v&state=%v", redirectUrl, code, state)
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		log.Printf("Unable to retrieve callback(%v): %v", url, err)
	} else {
		log.Printf("Callback with %v", resp.StatusCode)
	}
}

func (s *Server) HandleOauth2(w http.ResponseWriter, r *http.Request) {
	rcode := r.URL.Query().Get("code")

	log.Printf("Handling 2")

	if rcode != code {
		// Return a 500 error
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Code %v, is incorrect", rcode)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, authorizeResponse)
}

func (s *Server) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling user info")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{
  "meta": {
    "http_code": 200
  },
  "response": {
    "user": {
      "uid": 12345,
      "user_name": "fakeuser",
      "first_name": "Fake",
      "last_name": "User"
    }
  }
}`)
}

func main() {
	log.Printf("Launching fake untappd")
	s := &Server{}

	http.Handle("/oauth/authenticate", http.HandlerFunc(s.HandleOauth1))
	http.Handle("/oauth/authorize", http.HandlerFunc(s.HandleOauth2))
	http.Handle("/v4/beer/info/", http.HandlerFunc(s.HandleGetBeer))
	http.Handle("/v4/user/info", http.HandlerFunc(s.HandleUserInfo))
	http.Handle("/checkin/", http.HandlerFunc(s.HandleCheckin))
	http.Handle("/v4/user/checkins/", http.HandlerFunc(s.HandleCheckins))

	err := http.ListenAndServe(fmt.Sprintf(":%v", *port), nil)
	log.Fatalf("Beerkellar is unable to serve metrics: %v", err)

}
