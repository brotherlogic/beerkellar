package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

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
	err = baseGet(rUrl, resp)
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

type UserResponse struct {
	User UserUserResponse
}

type UserUserResponse struct {
	UserName string `json:"user_name"`
}

func (s Server) UpdateUser(ctx context.Context, user *pb.User) error {
	resp := &UserResponse{}
	err := get(fmt.Sprintf("/v4/user/info/%v", user.GetUsername()), resp)

	if err != nil {
		return err
	}

	user.Username = resp.User.UserName
	return nil
}

func (s Server) UpdateUserCheckins(ctx context.Context, user *pb.User) error {
	resp := &CheckinResponse{}
	err := get(fmt.Sprintf("/v4/user/checkins/", resp))

	if err != nil {
		return err
	}

	for _, checkin := range resp.Checkins.Items {
		dt, err := time.Parse(checkin.CreatedAt, "Sat, 13 Dec 2014 19:15:38 +0000")
		if err != nil {
			return err
		}
		d := &pb.Drink{
			BeerId: int64(checkin.BeerId),
			Score:  checkin.RatingScore,
			Date:   dt.Unix(),
		}
		err = s.db.SaveDrink(ctx, user.GetUserId(), d)
		if err != nil {
			return err
		}

		if checkin.CheckinId > user.GetLastCheckin() {
			user.LastCheckin = checkin.CheckinId
		}
	}

	return nil
}

type CheckinResponse struct {
	Checkins Checkins
}

type Checkins struct {
	Count int32
	Items []CheckinItems
}

type CheckinItems struct {
	CheckinId   int64  `json:"checkin_id"`
	CreatedAt   string `json:"created_at"`
	BeerId      int32  `json:"beer_id"`
	RatingScore int32  `json:"rating_score"`
}
