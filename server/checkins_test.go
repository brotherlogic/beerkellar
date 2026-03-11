package server

import (
	"context"
	"testing"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func TestPullCheckins(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   123,
		SizeFlOz: 12,
		Quantity: 12,
	})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	_, err = s.DrinkBeer(ctx, &pb.DrinkBeerRequest{BeerId: 123})
	if err != nil {
		t.Fatalf("Unable to mark beer as drunk: %v", err)
	}

	cellar, err := s.GetCellar(ctx, &pb.GetCellarRequest{})
	if err != nil {
		t.Fatalf("Unable to get cellar: %v", err)
	}

	if len(cellar.GetBeers()) != 11 {
		t.Errorf("Beer was not pulled from cellar: %v", cellar)
	}
}
