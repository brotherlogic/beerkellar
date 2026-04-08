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

func TestGetCellarReturnsEmptyForNewUser(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	r, err := s.GetCellar(ctx, &pb.GetCellarRequest{})
	if err != nil {
		t.Fatalf("Unable to get cellar: %v", err)
	}

	if len(r.GetBeers()) != 0 {
		t.Fatalf("Beers returned for empty cellar: %v", r)
	}
}

func TestGetDrunkReturnsEmptyForNewUser(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	r, err := s.GetDrunk(ctx, &pb.GetDrunkRequest{})
	if err != nil {
		t.Fatalf("Unable to get drunk records: %v", err)
	}

	if len(r.GetDrunk()) != 0 {
		t.Fatalf("Records returned for empty drunk archive: %v", r)
	}
}

func TestAddBeerRequiresSize(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   123,
		Quantity: 1,
		// SizeFlOz: 0 is default
	})
	if err == nil {
		t.Fatalf("Should have failed to add beer without size")
	}
}

func TestGetBeer_LeastRecentlyDrunk(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Add two beers
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   1,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer 1: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 1, Name: "Beer 1", Abv: 5.0})

	// Set specific date for first beer
	cellar, _ := s.db.GetCellar(ctx, 100)
	cellar.Entries[0].DateAdded = 1000
	s.db.SaveCellar(ctx, 100, cellar)

	_, err = s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   2,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer 2: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 2, Name: "Beer 2", Abv: 5.0})

	// Set specific date for second beer (older)
	cellar, _ = s.db.GetCellar(ctx, 100)
	cellar.Entries[1].DateAdded = 500
	s.db.SaveCellar(ctx, 100, cellar)

	// Initially, both have 0 last drunk date.
	// STRATEGY_LEAST_RECENTLY_DRUNK should pick the oldest in the cellar (beer 2).
	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK},
		},
	})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}
	if len(r.GetBeers()) == 0 || r.GetBeers()[0].GetId() != 2 {
		t.Errorf("Expected beer 2 (oldest), got %v", r.GetBeers())
	}

	// Now mark beer 2 as drunk recently
	s.db.SaveDrunk(ctx, 100, &pb.LastCheckins{
		LastCheckins: map[int64]int64{2: 1000},
	})

	// Now beer 1 should be picked (since it has 0 drunk date)
	r, err = s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK},
		},
	})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}
	if len(r.GetBeers()) == 0 || r.GetBeers()[0].GetId() != 1 {
		t.Errorf("Expected beer 1 (not drunk recently), got %v", r.GetBeers())
	}
}

func TestGetBeer_WeekdayLogic(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Add a heavy beer (3.54 units)
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   1,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer 1: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 1, Name: "Heavy", Abv: 10.0})

	// Add a light beer (1.41 units)
	_, err = s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   2,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer 2: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 2, Name: "Light", Abv: 4.0})

	// Test with weekday limit (2.5)
	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MaxUnits: 2.5},
		},
	})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}
	if len(r.GetBeers()) != 1 || r.GetBeers()[0].GetId() != 2 {
		t.Errorf("Expected light beer 2, got %v", r.GetBeers())
	}

	// Test without limit
	r, err = s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MaxUnits: 0},
		},
	})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}
	if len(r.GetBeers()) != 1 {
		t.Errorf("Expected a beer, got none")
	}
}

