package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type Untappd struct {
	baseAPIURL  string
	baseAuthURL string
	retAuthURL  string

	clientId     string
	clientSecret string
	accessToken  string
}

type strpass struct {
	Value string
}

func GetUntappd(api, auth, retAuth string, clientId, clientSecret string) *Untappd {
	return &Untappd{
		baseAPIURL:  api,
		baseAuthURL: auth,
		retAuthURL:  retAuth,

		clientId:     clientId,
		clientSecret: clientSecret,
	}
}

func (u *Untappd) Upgrade(accessToken string) *Untappd {
	return &Untappd{
		clientId:     u.clientId,
		clientSecret: u.clientSecret,
		accessToken:  u.accessToken,
	}
}

func (u *Untappd) get(urlSuffix string, obj interface{}) error {
	path := fmt.Sprintf("%v%v", u.baseAPIURL, urlSuffix)
	return u.baseGet(path, obj)
}

func (u *Untappd) baseGet(url string, obj interface{}) error {
	addition := fmt.Sprintf("client_id=%v&client_secret=%v", u.clientId, u.clientSecret)
	if u.accessToken != "" {
		addition = fmt.Sprintf("access_token=%v", u.accessToken)
	}
	if strings.Contains(url, "?") {
		url = fmt.Sprintf("%v&%v", url, addition)
	} else {
		url = fmt.Sprintf("%v?%v", url, addition)
	}

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

	log.Printf("READ %v", string(body))
	return json.Unmarshal(body, obj)
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

	rUrl := fmt.Sprintf("%voauth/authorize?client_id=%v&client_secret=%v&response_type=code&redirect_url=%v&code=%v",
		s.untappd.retAuthURL, s.clientId, s.clientSecret, s.redirectUrl, code)
	resp := &AuthResponse{}
	err = s.untappd.baseGet(rUrl, resp)
	if err != nil {
		log.Printf("Bad get: %v (%v)", err, rUrl)
		return nil, err
	}

	user.AccessToken = resp.Response.AccessToken
	err = s.db.SaveUser(ctx, user)
	return user, err
}

func (u *Untappd) getBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error) {
	resp := &BeerInfoResponse{}
	err := u.get(fmt.Sprintf("/v4/beer/info/%v", beerId), resp)
	log.Printf("BeerResponse: %v", err)
	if err != nil {
		return nil, err
	}
	return &pb.Beer{
		Id:      beerId,
		Name:    resp.Beer.BeerName,
		Abv:     resp.Beer.BeerAbv,
		Brewery: resp.Beer.Brewery.BreweryName,
	}, nil
}

type BreweryResponse struct {
	BreweryName string
}

type BeerResponse struct {
	BeerName    string  `json:"beer_name"`
	BeerAbv     float32 `json:"beer_abv"`
	Brewery     BreweryResponse
	RatingScore float32 `json:"rating_score"`
}

type BeerInfoResponse struct {
	Beer BeerResponse
}
