package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type Untappd struct {
	baseAPIURL  string
	baseAuthURL string
	retAuthURL  string
}

type strpass struct {
	Value string
}

func GetUntappd(api, auth, retAuth string) *Untappd {
	return &Untappd{
		baseAPIURL:  api,
		baseAuthURL: auth,
		retAuthURL:  retAuth,
	}
}

func (u *Untappd) get(urlSuffix string, obj interface{}) error {
	path := fmt.Sprintf("%v%v", u.baseAPIURL, urlSuffix)
	return baseGet(path, obj)
}

func baseGet(url string, obj interface{}) error {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("%v: %v", resp.StatusCode, string(body))
	}

	log.Printf("READ %v", len(string(body)))
	nobj := obj.(*strpass)
	nobj.Value = string(body)
	return nil
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

type AuthResponse struct {
	Response TokenResponse
}

func (s *Server) handleAuthResponse(ctx context.Context, u *Untappd, code, token string) (*pb.User, error) {
	log.Printf("Handling auth")
	user, err := s.db.GetUser(ctx, token)
	if err != nil {
		return nil, err
	}

	rUrl := fmt.Sprintf("%voauth/authorize/?client_id=%v&client_secret=%v&response_type=code&redirect_url=%v&code=%v",
		s.untappd.retAuthURL, s.clientId, s.clientSecret, s.redirectUrl, code)
	resp := &AuthResponse{}
	err = baseGet(rUrl, resp)
	if err != nil {
		log.Printf("Bad get: %v", err)
		return nil, err
	}

	user.AccessToken = resp.Response.AccessToken
	err = s.db.SaveUser(ctx, user)
	return user, err
}

func (u *Untappd) getBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error) {
	resp := &BeerInfoResponse{}
	err := u.get(fmt.Sprintf("/v4/beer/info/%v", beerId), resp)
	if err != nil {
		return nil, err
	}
	return &pb.Beer{
		Id:      beerId,
		Name:    resp.Bear.BeerName,
		Abv:     resp.Bear.BeerAbv,
		Brewery: resp.Bear.Brewery.BreweryName,
	}, nil
}

type BreweryResponse struct {
	BreweryName string
}

type BeerResponse struct {
	BeerName    string
	BeerAbv     float32
	Brewery     BreweryResponse
	RatingScore float32
}

type BeerInfoResponse struct {
	Bear BeerResponse
}
