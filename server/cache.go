package server

import (
	"context"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type BeerCache struct {
	beers []*pb.Beer
}

func (S Server) loadCache(ctx context.Context) (*BeerCache, error) {
	return &BeerCache{}, nil
}

func (b BeerCache) flushCache(ctx context.Context) error {
	return nil
}

func (b BeerCache) GetBeer(ctx context.Context, beerId int64) (*pb.Beer, error) {
	for _, beer := range b.beers {
		if beer.GetId() == beerId {
			return beer, nil
		}
	}

	return s.GetBeerFromUntappd(ctx, beerId)
}
