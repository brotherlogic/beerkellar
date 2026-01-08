package server

import (
	"context"
	"fmt"
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

func (s *Server) GetBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error) {
	return &pb.Beer{}, nil
}

func (s *Server) getBeer(ctx context.Context, beerId int64) (*pb.Beer, error) {
	cache, err := s.loadCache(ctx)
	if err != nil {
		return nil, err
	}
	return cache.GetBeer(ctx, beerId)
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
	return &pb.GetLoginResponse{Url: fmt.Sprintf("https://untappd.com/oauth/authenticate/?client_id=%v&response_type=code&redirect_url=%v", s.clientId, s.redirectUrl)}, nil
}
