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

func TestGetDrunk(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Add a beer to cellar
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   123,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 123, Name: "Test Beer", Abv: 5.0})

	// Simulate a checkin matching the cellar entry
	ut := s.untappd.(*TestUntappd)
	checkinDate := time.Now().Unix()
	ut.checkins = []*pb.Checkin{
		{CheckinId: 1, BeerId: 123, Date: checkinDate, Rating: 4},
	}

	// Refresh user to process checkin
	_, err = s.RefreshUser(ctx, &pb.RefreshUserRequest{Username: "testuser"})
	if err != nil {
		t.Fatalf("RefreshUser failed: %v", err)
	}

	// Get drunk beers
	r, err := s.GetDrunk(ctx, &pb.GetDrunkRequest{Count: 10})
	if err != nil {
		t.Fatalf("GetDrunk failed: %v", err)
	}

	if len(r.GetDrunk()) != 1 {
		t.Fatalf("Expected 1 drunk beer, got %v", len(r.GetDrunk()))
	}

	if r.GetDrunk()[0].GetBeerId() != 123 {
		t.Errorf("Wrong beer ID: %v", r.GetDrunk()[0].GetBeerId())
	}

	if r.GetDrunk()[0].GetSizeFlOz() != 12 {
		t.Errorf("Size not captured: %v", r.GetDrunk()[0].GetSizeFlOz())
	}

	// Units should be conversion(12) * 5.0 = (12 * 0.029574) * 5.0 = 1.77444
	if r.GetDrunk()[0].GetUnits() < 1.7 || r.GetDrunk()[0].GetUnits() > 1.8 {
		t.Errorf("Wrong units: %v", r.GetDrunk()[0].GetUnits())
	}
}

func TestRefreshUser_NilMapPanic(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Pre-save a LastCheckins object with a nil/empty map to trigger the scenario.
	// In proto3, if we don't initialize the map, it will be nil when unmarshaled.
	err := s.db.SaveDrunk(ctx, 100, &pb.LastCheckins{
		Username: "testuser",
	})
	if err != nil {
		t.Fatalf("Failed to save drunk: %v", err)
	}

	// Add a beer to cellar so RefreshUser has something to do if checkins are found
	_, err = s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   123,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	// Simulate a checkin
	ut := s.untappd.(*TestUntappd)
	ut.checkins = []*pb.Checkin{
		{CheckinId: 1, BeerId: 123, Date: time.Now().Unix(), Rating: 4},
	}

	// This should NOT panic now
	_, err = s.RefreshUser(ctx, &pb.RefreshUserRequest{Username: "testuser"})
	if err != nil {
		t.Fatalf("RefreshUser failed: %v", err)
	}
}

func TestGetCellar_WithUnits(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Add a beer with 12 fl oz and 5.0% ABV
	// 12 * 0.029574 * 5.0 = 1.77444
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   123,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 123, Name: "Test Beer", Abv: 5.0})

	r, err := s.GetCellar(ctx, &pb.GetCellarRequest{})
	if err != nil {
		t.Fatalf("Unable to get cellar: %v", err)
	}

	if len(r.GetBeers()) != 1 {
		t.Fatalf("Expected 1 beer, got %v", len(r.GetBeers()))
	}

	units := r.GetBeers()[0].GetUnits()
	if units < 1.7 || units > 1.8 {
		t.Errorf("Wrong units returned: %v (expected ~1.77)", units)
	}
}

func TestGetBeer_UnitsAndFiltering(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Add a heavy beer (12oz, 10% ABV = 3.54 units)
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   1,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer 1: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 1, Name: "Heavy", Abv: 10.0})

	// Add a light beer (12oz, 5% ABV = 1.77 units)
	_, err = s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   2,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer 2: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 2, Name: "Light", Abv: 5.0})

	// Call GetBeer with MaxUnits = 2.5
	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MaxUnits: 2.5, Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK},
		},
	})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}

	if len(r.GetBeers()) == 0 {
		t.Fatalf("No beers returned")
	}

	for _, beer := range r.GetBeers() {
		if beer.GetId() == 1 {
			t.Errorf("Returned beer 1 which is over 2.5 units (3.54 units)")
		}
		if beer.GetUnits() == 0 {
			t.Errorf("Units not populated for beer %v", beer.GetId())
		}
		expectedUnits := float32(12) * 0.029574 * 5.0
		if beer.GetId() == 2 && (beer.GetUnits() < expectedUnits-0.01 || beer.GetUnits() > expectedUnits+0.01) {
			t.Errorf("Wrong units for beer 2: %v (expected ~%v)", beer.GetUnits(), expectedUnits)
		}
	}
}

func TestGoogleTaskThreshold(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)
	// Create user
	user := &pb.User{
		UserId:   100,
		Username: "testuser",
		Auth:     "auth-token",
		State:    pb.User_STATE_AUTHORIZED,
	}
	s.db.SaveUser(ctx, user)
	md := metadata.Pairs("auth-token", "auth-token")
	ctx = metadata.NewIncomingContext(ctx, md)

	// Save a low ABV beer definition
	s.db.SaveBeer(ctx, &pb.Beer{Id: 1, Name: "Light Beer", Abv: 4.0}) // ~1.4 units for 12oz

	// Add 4 weekday beers to go above the threshold
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   1,
		SizeFlOz: 12,
		Quantity: 4,
	})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	user, _ = s.db.GetUser(ctx, "auth-token")
	if user.GetGoogleTaskActive() {
		t.Errorf("GoogleTaskActive should be false initially")
	}

	// Drink 1 beer, dropping the count to 3 (< 4 threshold)
	_, err = s.DrinkBeer(ctx, &pb.DrinkBeerRequest{BeerId: 1})
	if err != nil {
		t.Fatalf("Unable to drink beer: %v", err)
	}

	user, _ = s.db.GetUser(ctx, "auth-token")
	if !user.GetGoogleTaskActive() {
		t.Errorf("GoogleTaskActive should be true after dropping below threshold")
	}

	// Add 1 beer, pushing the count back to 4
	_, err = s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   1,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Unable to add beer again: %v", err)
	}

	user, _ = s.db.GetUser(ctx, "auth-token")
	if user.GetGoogleTaskActive() {
		t.Errorf("GoogleTaskActive should be false after adding back above threshold")
	}
}
