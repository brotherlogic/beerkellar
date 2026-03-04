package server

import (
	"context"
	"testing"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc/metadata"
)

func getTestServer(ctx context.Context) *Server {
	db := NewTestDatabase(ctx)
	ut := GetTestUntappd()
	return &Server{
		db:      db,
		untappd: ut,

		q: NewQueue(),
	}
}

func GetTestContext(ctx context.Context, deadline time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, deadline)
	ctx = metadata.AppendToOutgoingContext(context.Background(),
		"auth-token",
		"testuser")
	return ctx, cancel
}

func TestGetBeer(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   123,
		SizeFlOz: 12,
		Quantity: 2,
	})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}
	s.q.Flush()

	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{Requirements: []*pb.BeerRequirement{{}}})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}

	if len(r.GetBeers()) == 0 {
		t.Fatalf("No beers returned: %v", r)
	}

	if r.GetBeers()[0].GetId() != 123 {
		t.Errorf("Wrong beer returned: %v", r)
	}
}

func TestGetBeerWithNoMulitples(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   123,
		SizeFlOz: 12,
		Quantity: 2,
	})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}
	s.q.Flush()

	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{NoRepeat: true, Requirements: []*pb.BeerRequirement{{}, {}}})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}

	if len(r.GetBeers()) != 1 {
		t.Fatalf("No beers returned (or wrong number): %v", r)
	}

	if r.GetBeers()[0].GetId() != 123 {
		t.Errorf("Wrong beer returned: %v", r)
	}
}
