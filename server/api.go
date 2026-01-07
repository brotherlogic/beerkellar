package server

import (
	"context"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"golang.org/x/tools/godoc/redirect"

	pstore_client "github.com/brotherlogic/pstore/client"
)

const (
	CONFIG = "github.com/brotherlogic/beerkellar/config"
	CACHE  = "github.com/brotherlogic/beerkellar/cache"
)

type Server struct {
	client pstore_client.PStoreClient
	string clientId
	string clientSecret
	string redirectUrl
}

func NewServer() *Server {
	return &Server{
		clientId: clientId,
		clientSecret: clientSecret,
		redictectUrl: redirectUrl,
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

func (s *Server) AddBeer(ctx context.Context, req *pb.AddBeerRequest) (*pb.AddBeerResponse, error) {
	cellar, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}

	for _ = range req.GetQuantity() {
		cellar.Entries = append(cellar.Entries,
			&pb.CellarEntry{
				BeerId:    req.GetBeerId(),
				SizeFlOz:  req.GetSizeFlOz(),
				DateAdded: time.Now().Unix(),
			})
	}

	return &pb.AddBeerResponse{}, nil
}

func (s *Server) GetLogin(ctx context.Context, req *pb.GetLoginRequest) (*pb.GetLoginResponse, error) {
	return fmt.Sprintf("https://untappd.com/oauth/authenticate/?client_id=%v&response_type=code&redirect_url=%v", s.clientId, s.redirectUrl)
")
}