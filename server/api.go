package server

import (
	"context"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) loadConfig(ctx context.Context) (*pb.Cellar, error) {
	return &pb.Cellar{}, nil
}

func (S Server) loadCache(ctx context.Context) (*pb.BeerCache, error) {
	return &pb.BeerCache{}, nil
}

func (s *Server) GetBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error) {
	return &pb.Beer{}, nil
}

func (s *Server) getBeer(ctx context.Context, beerId int64) (*pb.Beer, error) {
	cache, err := s.loadCache(ctx)
	if err != nil {
		return nil, err
	}

	for _, entry := range cache.GetBeers() {
		if entry.GetId() == beerId {
			return entry, nil
		}
	}

	return s.GetBeerFromUntappd(ctx, beerId)
}

func (s *Server) AddBeer(ctx context.Context, req *pb.AddBeerRequest) (*pb.AddBeerResponse, error) {
	cellar, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}

	beer, err := s.getBeer(ctx, req.GetBeerId())
	if err != nil {
		return nil, err
	}

	for _ := range req.GetQuantity() {
		cellar.Beers = append(cellar.Beers, &pb.Beer{
			Id:          req.GetBeerId(),
			Brewery:     beer.GetBrewery(),
			Abv:         beer.GetAbv(),
			Name:        beer.GetName(),
			GlobalScore: beer.GetGlobalScore(),
		})
	}
}
