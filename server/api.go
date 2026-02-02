package server

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pstore_client "github.com/brotherlogic/pstore/client"
)

const (
	CONFIG = "github.com/brotherlogic/beerkellar/config"
	CACHE  = "github.com/brotherlogic/beerkellar/cache"
)

type Server struct {
	client pstore_client.PStoreClient
	db     Database

	clientId     string
	clientSecret string
	redirectUrl  string

	untappd *Untappd
}

func NewServer(clientId, clientSecret, redirectUrl string, db Database) *Server {
	return &Server{
		clientId:     clientId,
		clientSecret: clientSecret,
		redirectUrl:  redirectUrl,

		db: db,
	}
}

func (s *Server) loadConfig(ctx context.Context) (*pb.Cellar, error) {
	return &pb.Cellar{}, nil
}

func (s *Server) getBeer(ctx context.Context, beerId int64) (*pb.Beer, error) {
	beer, err := s.db.GetBeer(ctx, beerId)
	if err == nil {
		return beer, nil
	}

	// Cache miss - call out to Untappd
	return s.untappd.getBeerFromUntappd(ctx, beerId)
}

func GetContextKey(ctx context.Context) (string, error) {
	md, found := metadata.FromIncomingContext(ctx)
	if found {
		if _, ok := md["auth-token"]; ok {
			idt := md["auth-token"][0]

			if idt != "" {
				return idt, nil
			}
		}
	}

	md, found = metadata.FromOutgoingContext(ctx)
	if found {
		if _, ok := md["auth-token"]; ok {
			idt := md["auth-token"][0]

			if idt != "" {
				return idt, nil
			}
		}
	}

	return "", status.Errorf(codes.NotFound, "Could not extract token from incoming or outgoing")
}

func (s *Server) getUser(ctx context.Context) (*pb.User, error) {
	key, err := GetContextKey(ctx)
	if err != nil {
		return nil, err
	}

	return s.db.GetUser(ctx, key)
}

func (s *Server) AddBeer(ctx context.Context, req *pb.AddBeerRequest) (*pb.AddBeerResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}
	cellar, err := s.db.GetCellar(ctx, user.GetUsername())
	if err != nil {
		// OutOfRange is cannot be found in DB
		if status.Code(err) == codes.OutOfRange {
			cellar = &pb.Cellar{}
		} else {
			return nil, err
		}
	}

	for _ = range req.GetQuantity() {
		cellar.Entries = append(cellar.Entries,
			&pb.CellarEntry{
				BeerId:    req.GetBeerId(),
				SizeFlOz:  req.GetSizeFlOz(),
				DateAdded: time.Now().Unix(),
			})
	}

	return &pb.AddBeerResponse{}, s.db.SaveCellar(ctx, user.GetUsername(), cellar)
}

func (s *Server) GetLogin(ctx context.Context, req *pb.GetLoginRequest) (*pb.GetLoginResponse, error) {
	tmpToken := fmt.Sprintf("%v-%v", time.Now().UnixNano(), rand.Int63())
	user := &pb.User{
		Auth: tmpToken,
	}
	err := s.db.SaveUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return &pb.GetLoginResponse{
		Url:  fmt.Sprintf("https://untappd.com/oauth/authenticate/?client_id=%v&response_type=code&redirect_url=%v&state=%v", s.clientId, s.redirectUrl, user.GetAuth()),
		Code: user.GetAuth(),
	}, nil
}

func (s *Server) GetAuthToken(ctx context.Context, req *pb.GetAuthTokenRequest) (*pb.GetAuthTokenResponse, error) {
	user, err := s.db.GetUser(ctx, req.GetCode())
	if err != nil {
		return nil, err
	}

	if len(user.GetAccessToken()) > 0 {
		return &pb.GetAuthTokenResponse{}, nil
	}
	return &pb.GetAuthTokenResponse{}, status.Errorf(codes.NotFound, "User is not fully authenticated")
}

func (s *Server) GetBeer(ctx context.Context, req *pb.GetBeerRequest) (*pb.GetBeerResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}
	cellar, err := s.db.GetCellar(ctx, user.GetUsername())
	if err != nil {
		// OutOfRange is cannot be found in DB
		if status.Code(err) == codes.OutOfRange {
			cellar = &pb.Cellar{}
		} else {
			return nil, err
		}
	}

	var beers []*pb.Beer
	addedDate := make(map[int64]int64)
	for _, entry := range cellar.GetEntries() {
		beer, err := s.getBeer(ctx, entry.GetBeerId())
		if err != nil {
			return nil, err
		}

		if date, ok := addedDate[beer.GetId()]; !ok || entry.GetDateAdded() < date {
			addedDate[beer.GetId()] = entry.GetDateAdded()
		} else {

		}

		units := convertToLitres(entry.GetSizeFlOz()) * beer.GetAbv()
		if units < float32(req.GetMaxUnits()) {
			beers = append(beers, beer)
		}
	}

	// Out of the beers - pick the oldest
	oldest := beers[0]
	for _, beer := range beers {
		if addedDate[beer.GetId()] < addedDate[oldest.GetId()] {
			oldest = beer
		}
	}

	return &pb.GetBeerResponse{Beer: oldest}, nil
}

func convertToLitres(flOz int32) float32 {
	return float32(flOz) * 0.029574
}

func (s *Server) Healthy(_ context.Context, _ *pb.HealthyRequest) (*pb.HealthyResponse, error) {
	return &pb.HealthyResponse{}, nil
}
