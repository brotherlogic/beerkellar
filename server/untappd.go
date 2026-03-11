package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type UntappdAPI interface {
	getBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error)
	Upgrade(at string) UntappdAPI
	getBaseAuthURL() string
	handleAuthResponse(ctx context.Context, db Database, code, token string) (*pb.User, error)
	GetCheckins(ctx context.Context) ([]*pb.Checkin, error)
}

type Untappd struct {
	baseAPIURL  string
	baseAuthURL string
	retAuthURL  string
	redirectURL string

	clientId     string
	clientSecret string
	accessToken  string
}

type strpass struct {
	Value string
}

type TestUntappd struct{}

func GetTestUntappd() UntappdAPI {
	return &TestUntappd{}
}

func (tu *TestUntappd) Upgrade(_ string) UntappdAPI {
	return tu
}

func (_ *TestUntappd) getBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error) {
	log.Printf("Getting Beer: %v", beerId)
	return &pb.Beer{Id: beerId, Abv: 4.5}, nil
}

func (_ *TestUntappd) getBaseAuthURL() string {
	return ""
}

func (_ *TestUntappd) GetCheckins(ctx context.Context) ([]*pb.Checkin, error) {
	return []*pb.Checkin{}, nil
}

func (_ *TestUntappd) handleAuthResponse(ctx context.Context, db Database, code, token string) (*pb.User, error) {
	return &pb.User{}, nil
}

func GetUntappd(api, auth, retAuth string, clientId, clientSecret, redirectURL string) *Untappd {
	return &Untappd{
		baseAPIURL:  api,
		baseAuthURL: auth,
		retAuthURL:  retAuth,
		redirectURL: redirectURL,

		clientId:     clientId,
		clientSecret: clientSecret,
	}
}

func (u *Untappd) Upgrade(accessToken string) UntappdAPI {
	return &Untappd{
		baseAPIURL:   u.baseAPIURL,
		clientId:     u.clientId,
		clientSecret: u.clientSecret,
		accessToken:  accessToken,
	}
}

func (u *Untappd) get(urlSuffix string, obj interface{}) error {
	path := fmt.Sprintf("%v%v", u.baseAPIURL, urlSuffix)
	return u.baseGet(path, obj)
}

func (u *Untappd) baseGet(url string, obj interface{}) error {
	log.Printf("Huh: %v", url)
	addition := fmt.Sprintf("client_id=%v&client_secret=%v", u.clientId, u.clientSecret)
	if u.accessToken != "" {
		addition = fmt.Sprintf("access_token=%v", u.accessToken)
	}
	if strings.Contains(url, "?") {
		url = fmt.Sprintf("%v&%v", url, addition)
	} else {
		url = fmt.Sprintf("%v?%v", url, addition)
	}

	log.Printf("Reading %v", url)
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

	return json.Unmarshal(body, obj)
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

type AuthResponse struct {
	Response TokenResponse
}

func (u *Untappd) getBaseAuthURL() string {
	return u.baseAuthURL
}

func (u *Untappd) handleAuthResponse(ctx context.Context, db Database, code, token string) (*pb.User, error) {
	log.Printf("Handling auth")
	user, err := db.GetUser(ctx, token)
	if err != nil {
		return nil, err
	}

	rUrl := fmt.Sprintf("%voauth/authorize?client_id=%v&client_secret=%v&response_type=code&redirect_url=%v&code=%v",
		u.retAuthURL, u.clientId, u.clientSecret, u.redirectURL, code)
	resp := &AuthResponse{}
	err = u.baseGet(rUrl, resp)
	if err != nil {
		log.Printf("Bad get: %v (%v)", err, rUrl)
		return nil, err
	}

	user.AccessToken = resp.Response.AccessToken
	err = db.SaveUser(ctx, user)
	return user, err
}

type CheckinResponse struct {
	Checkins []Checkin
}

type Checkin struct {
	CheckinId   int    `json:"checkin_id`
	CreatedAt   string `json:"created_at`
	RatingScore int    `json:"rating_score`
	Beer        BeerResponse
}

func parseDate(dstr string) int64 {
	val, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", dstr)
	if err != nil {
		panic(err)
	}
	return val.Unix()
}

func (u *Untappd) GetCheckins(ctx context.Context) ([]*pb.Checkin, error) {
	resp := &CheckinResponse{}
	err := u.get("/v4/user/checkins/", resp)
	if err != nil {
		return nil, err
	}

	var checkins []*pb.Checkin
	for _, c := range resp.Checkins {
		checkins = append(checkins, &pb.Checkin{
			CheckinId: int64(c.CheckinId),
			BeerId:    int64(c.Beer.Bid),
			Rating:    int32(c.RatingScore),
			Date:      parseDate(c.CreatedAt),
		})
	}

	return []*pb.Checkin{}, nil
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
	Bid         int     `json:bid`
}

type BeerInfoResponse struct {
	Beer BeerResponse
}
