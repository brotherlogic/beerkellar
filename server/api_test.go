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

func TestAuthFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// 1. GetLogin should create a user in STATE_CREATED
	loginResp, err := s.GetLogin(ctx, &pb.GetLoginRequest{})
	if err != nil {
		t.Fatalf("GetLogin failed: %v", err)
	}

	user, err := s.db.GetUser(ctx, loginResp.GetCode())
	if err != nil {
		t.Fatalf("Unable to get user: %v", err)
	}
	if user.GetState() != pb.User_STATE_LOGGING_IN {
		t.Errorf("Expected state LOGGING_IN, got %v", user.GetState())
	}

	// 2. Simulate callback which sets STATE_AUTHENTICATED
	user.AccessToken = "access-token"
	user.State = pb.User_STATE_LOGGED_IN
	err = s.db.SaveUser(ctx, user)
	if err != nil {
		t.Fatalf("SaveUser failed: %v", err)
	}

	// 3. GetAuthToken should fetch info and set STATE_LOGGED_IN
	_, err = s.GetAuthToken(ctx, &pb.GetAuthTokenRequest{Code: loginResp.GetCode()})
	if err != nil {
		t.Fatalf("GetAuthToken failed: %v", err)
	}

	user, err = s.db.GetUser(ctx, loginResp.GetCode())
	if err != nil {
		t.Fatalf("Unable to get user: %v", err)
	}
	if user.GetState() != pb.User_STATE_AUTHORIZED {
		t.Errorf("Expected state AUTHORIZED, got %v", user.GetState())
	}
}
