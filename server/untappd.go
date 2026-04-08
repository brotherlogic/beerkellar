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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UntappdAPI interface {
	getBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error)
	Upgrade(at string) UntappdAPI
	getBaseAuthURL() string
	handleAuthResponse(ctx context.Context, db Database, code, token string) (*pb.User, error)
	GetCheckins(ctx context.Context) ([]*pb.Checkin, error)
	Checkin(ctx context.Context, beerId int64) error
	GetUserInfo(ctx context.Context) (string, int64, error)
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

type TestUntappd struct {
	checkins []*pb.Checkin
}

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

func (tu *TestUntappd) GetCheckins(ctx context.Context) ([]*pb.Checkin, error) {
	return tu.checkins, nil
}

func (tu *TestUntappd) Checkin(ctx context.Context, beerId int64) error {
	tu.checkins = append(tu.checkins, &pb.Checkin{CheckinId: time.Now().Unix(), Date: time.Now().Unix(), BeerId: beerId})
	return nil
}

func (_ *TestUntappd) handleAuthResponse(ctx context.Context, db Database, code, token string) (*pb.User, error) {
	return &pb.User{}, nil
}

func (_ *TestUntappd) GetUserInfo(ctx context.Context) (string, int64, error) {
	return "testuser", 123, nil
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

	log.Printf("READ %v", string(body))

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

func (u *Untappd) Checkin(ctx context.Context, beerId int64) error {
	return status.Errorf(codes.Unimplemented, "Not supported by vanilla untappd")
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
	user.State = pb.User_STATE_LOGGED_IN
	err = db.SaveUser(ctx, user)
	return user, err
}

type UserInfo struct {
	Uid      int64  `json:"uid"`
	UserName string `json:"user_name"`
}

type UserInfoResponse struct {
	Response struct {
		User UserInfo `json:"user"`
	} `json:"response"`
}

func (u *Untappd) GetUserInfo(ctx context.Context) (string, int64, error) {
	resp := &UserInfoResponse{}
	err := u.get("/v4/user/info", resp)
	if err != nil {
		return "", 0, err
	}

	return resp.Response.User.UserName, resp.Response.User.Uid, nil
}

type CheckinResponse struct {
	Response struct {
		Checkins []Checkin `json:"items"`
	} `json:"response"`
}

type Checkin struct {
	CheckinId   int    `json:"checkin_id"`
	CreatedAt   string `json:"created_at"`
	RatingScore int    `json:"rating_score"`
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

	log.Printf("Checkins resp: %+v", resp)

	var checkins []*pb.Checkin
	for _, c := range resp.Response.Checkins {
		checkins = append(checkins, &pb.Checkin{
			CheckinId: int64(c.CheckinId),
			BeerId:    int64(c.Beer.Bid),
			Rating:    int32(c.RatingScore),
			Date:      parseDate(c.CreatedAt),
		})
	}

	return checkins, nil
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
		Name:    resp.Response.Beer.BeerName,
		Abv:     resp.Response.Beer.BeerAbv,
		Brewery: resp.Response.Beer.Brewery.BreweryName,
	}, nil
}

type BreweryResponse struct {
	BreweryName string `json:"brewery_name"`
}

type BeerResponse struct {
	BeerName    string          `json:"beer_name"`
	BeerAbv     float32         `json:"beer_abv"`
	Brewery     BreweryResponse `json:"brewery"`
	RatingScore float32         `json:"rating_score"`
	Bid         int             `json:"bid"`
}

type BeerInfoResponse struct {
	Response struct {
		Beer BeerResponse `json:"beer"`
	} `json:"response"`
}
