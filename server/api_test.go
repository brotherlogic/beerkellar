package server

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

	// Test with weekday limit (3.5)
	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MaxUnits: 3.5},
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

	// Call GetBeer with MaxUnits = 3
	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MaxUnits: 3, Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK},
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
			t.Errorf("Returned beer 1 which is over 3 units (3.54 units)")
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

func TestGetWeekdayBeerCount(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Add a user
	user := &pb.User{
		Auth:   "testuser",
		UserId: 12345,
		State:  pb.User_STATE_AUTHORIZED,
	}
	s.db.SaveUser(ctx, user)

	// 12oz, 9% ABV = 12 * 0.029574 * 9.0 = 3.19 units (should count as weekday under the new threshold of 3.5, but not under the old threshold of 3)
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   1,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("AddBeer failed: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 1, Name: "Medium-Heavy", Abv: 9.0})

	// 12oz, 10% ABV = 12 * 0.029574 * 10.0 = 3.548 units (should not count as weekday under either threshold)
	_, err = s.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   2,
		SizeFlOz: 12,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("AddBeer failed: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 2, Name: "Very Heavy", Abv: 10.0})

	count, err := s.getWeekdayBeerCount(ctx, user.GetUserId())
	if err != nil {
		t.Fatalf("getWeekdayBeerCount failed: %v", err)
	}

	// Under the new threshold of 3.5, the 9% beer should count, but the 10% should not. So count must be 1.
	if count != 1 {
		t.Errorf("Expected weekday beer count of 1, got %v (since 3.19 units < 3.5, but currently using old threshold < 3.0)", count)
	}
}

func TestGetBeer_MinUnitsAndFallback(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)

	// Add a light beer (12oz, 4% ABV = ~1.42 units)
	_, err := s.AddBeer(ctx, &pb.AddBeerRequest{BeerId: 1, SizeFlOz: 12, Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 1, Name: "Light", Abv: 4.0})

	// Add a medium beer (12oz, 6% ABV = ~2.13 units)
	_, err = s.AddBeer(ctx, &pb.AddBeerRequest{BeerId: 2, SizeFlOz: 12, Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add: %v", err)
	}
	s.db.SaveBeer(ctx, &pb.Beer{Id: 2, Name: "Medium", Abv: 6.0})

	// Scenario 1: MinUnits = 2.0. Should return Medium beer (BeerId = 2).
	r, err := s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MinUnits: 2.0},
		},
	})
	if err != nil || len(r.GetBeers()) == 0 {
		t.Fatalf("Failed to get beer: %v", err)
	}
	if r.GetBeers()[0].GetId() != 2 {
		t.Errorf("Expected beer 2, got %v", r.GetBeers()[0].GetId())
	}

	// Scenario 2: MinUnits = 3.0. No beer has >= 3 units. Fallback should return highest units (Medium beer, BeerId = 2).
	r2, err := s.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MinUnits: 3.0},
		},
	})
	if err != nil || len(r2.GetBeers()) == 0 {
		t.Fatalf("Failed to get beer: %v", err)
	}
	if r2.GetBeers()[0].GetId() != 2 {
		t.Errorf("Expected fallback to beer 2 (highest units), got %v", r2.GetBeers()[0].GetId())
	}
}

type errorInjectingDB struct {
	Database
	saveUserErr   error
	saveCellarErr error
}

func (db *errorInjectingDB) SaveUser(ctx context.Context, user *pb.User) error {
	if db.saveUserErr != nil {
		return db.saveUserErr
	}
	return db.Database.SaveUser(ctx, user)
}

func (db *errorInjectingDB) SaveCellar(ctx context.Context, userId int64, cellar *pb.Cellar) error {
	if db.saveCellarErr != nil {
		return db.saveCellarErr
	}
	return db.Database.SaveCellar(ctx, userId, cellar)
}

type blockingCheckinsUntappd struct {
	*TestUntappd
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (u *blockingCheckinsUntappd) Upgrade(_ string) UntappdAPI {
	return u
}

func (u *blockingCheckinsUntappd) GetCheckins(ctx context.Context) ([]*pb.Checkin, error) {
	u.once.Do(func() {
		close(u.started)
	})
	select {
	case <-u.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return u.TestUntappd.GetCheckins(ctx)
}

func TestGetCellar_ReturnsParallelEntries(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)
	s.q = NewTestQueue()

	if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 100, Name: "Beer A", Brewery: "Brewery A", Abv: 5.0}); err != nil {
		t.Fatalf("SaveBeer failed: %v", err)
	}

	expected := []*pb.CellarEntry{
		{BeerId: 100, SizeFlOz: 12, DateAdded: 1000},
		{BeerId: 100, SizeFlOz: 16, DateAdded: 2000},
		{BeerId: 200, SizeFlOz: 22, DateAdded: 3000}, // uncached beer
	}
	if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{Entries: expected}); err != nil {
		t.Fatalf("SaveCellar failed: %v", err)
	}

	resp, err := s.GetCellar(ctx, &pb.GetCellarRequest{})
	if err != nil {
		t.Fatalf("GetCellar failed: %v", err)
	}

	if len(resp.GetBeers()) != len(expected) {
		t.Fatalf("len(Beers) = %d, want %d", len(resp.GetBeers()), len(expected))
	}
	if len(resp.GetEntries()) != len(resp.GetBeers()) {
		t.Fatalf("len(Entries) = %d, want %d (parallel with Beers)", len(resp.GetEntries()), len(resp.GetBeers()))
	}

	for i, want := range expected {
		gotEntry := resp.GetEntries()[i]
		gotBeer := resp.GetBeers()[i]
		if gotBeer.GetId() != want.GetBeerId() {
			t.Errorf("Beers[%d].Id = %d, want %d", i, gotBeer.GetId(), want.GetBeerId())
		}
		if gotEntry.GetBeerId() != want.GetBeerId() ||
			gotEntry.GetSizeFlOz() != want.GetSizeFlOz() ||
			gotEntry.GetDateAdded() != want.GetDateAdded() {
			t.Errorf("Entries[%d] = %+v, want %+v", i, gotEntry, want)
		}
	}
}

func TestRemoveBeer_SuccessAndDuplicatePreservation(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)
	s.q = NewTestQueue()

	if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 100, Name: "Dup Beer", Brewery: "Brewery", Abv: 6.0}); err != nil {
		t.Fatalf("SaveBeer 100 failed: %v", err)
	}
	if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 200, Name: "Other Beer", Brewery: "Brewery", Abv: 7.0}); err != nil {
		t.Fatalf("SaveBeer 200 failed: %v", err)
	}

	initial := []*pb.CellarEntry{
		{BeerId: 100, SizeFlOz: 12, DateAdded: 1000},
		{BeerId: 100, SizeFlOz: 16, DateAdded: 2000},
		{BeerId: 100, SizeFlOz: 22, DateAdded: 3000},
		{BeerId: 200, SizeFlOz: 12, DateAdded: 4000},
	}
	if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{Entries: initial}); err != nil {
		t.Fatalf("SaveCellar failed: %v", err)
	}

	_, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{
		Index:     1,
		BeerId:    100,
		SizeFlOz:  16,
		DateAdded: 2000,
	})
	if err != nil {
		t.Fatalf("RemoveBeer failed: %v", err)
	}

	want := []*pb.CellarEntry{
		{BeerId: 100, SizeFlOz: 12, DateAdded: 1000},
		{BeerId: 100, SizeFlOz: 22, DateAdded: 3000},
		{BeerId: 200, SizeFlOz: 12, DateAdded: 4000},
	}

	cellar, err := s.db.GetCellar(ctx, 100)
	if err != nil {
		t.Fatalf("db.GetCellar failed: %v", err)
	}
	if len(cellar.GetEntries()) != len(want) {
		t.Fatalf("db len(Entries) = %d, want %d", len(cellar.GetEntries()), len(want))
	}
	for i, w := range want {
		got := cellar.GetEntries()[i]
		if got.GetBeerId() != w.GetBeerId() || got.GetSizeFlOz() != w.GetSizeFlOz() || got.GetDateAdded() != w.GetDateAdded() {
			t.Errorf("db Entries[%d] = %+v, want %+v", i, got, w)
		}
	}

	resp, err := s.GetCellar(ctx, &pb.GetCellarRequest{})
	if err != nil {
		t.Fatalf("GetCellar failed: %v", err)
	}
	if len(resp.GetEntries()) != len(want) {
		t.Fatalf("GetCellar len(Entries) = %d, want %d", len(resp.GetEntries()), len(want))
	}
	for i, w := range want {
		got := resp.GetEntries()[i]
		if got.GetBeerId() != w.GetBeerId() || got.GetSizeFlOz() != w.GetSizeFlOz() || got.GetDateAdded() != w.GetDateAdded() {
			t.Errorf("GetCellar Entries[%d] = %+v, want %+v", i, got, w)
		}
	}
}

func TestRemoveBeer_NoUntappdOrDrinkHistorySideEffects(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)
	s.q = NewTestQueue()

	if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 123, Name: "Test Beer", Brewery: "Test Brewery", Abv: 5.0}); err != nil {
		t.Fatalf("SaveBeer failed: %v", err)
	}
	if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{
		Entries: []*pb.CellarEntry{{BeerId: 123, SizeFlOz: 12, DateAdded: 1000}},
	}); err != nil {
		t.Fatalf("SaveCellar failed: %v", err)
	}

	_, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{
		Index:     0,
		BeerId:    123,
		SizeFlOz:  12,
		DateAdded: 1000,
	})
	if err != nil {
		t.Fatalf("RemoveBeer failed: %v", err)
	}

	if got := len(s.untappd.(*TestUntappd).checkins); got != 0 {
		t.Errorf("TestUntappd.checkins len = %d, want 0", got)
	}
	drunkResp, err := s.GetDrunk(ctx, &pb.GetDrunkRequest{Count: 10})
	if err != nil {
		t.Fatalf("GetDrunk failed: %v", err)
	}
	if len(drunkResp.GetDrunk()) != 0 {
		t.Errorf("GetDrunk len = %d, want 0", len(drunkResp.GetDrunk()))
	}
	if _, err := s.db.GetDrunk(ctx, 100); status.Code(err) != codes.NotFound {
		t.Errorf("db.GetDrunk err = %v, want NotFound", err)
	}

	// Verify pre-existing drink history is also preserved untouched.
	if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 999, Name: "Old Beer", Brewery: "Old Brewery", Abv: 6.0}); err != nil {
		t.Fatalf("SaveBeer 999 failed: %v", err)
	}
	if err := s.db.SaveCheckin(ctx, 100, &pb.Checkin{CheckinId: 1, BeerId: 999, Date: 5555, Rating: 4, SizeFlOz: 12}); err != nil {
		t.Fatalf("SaveCheckin failed: %v", err)
	}
	if err := s.db.SaveDrunk(ctx, 100, &pb.LastCheckins{
		Username:     "testuser",
		LastCheckins: map[int64]int64{999: 5555},
	}); err != nil {
		t.Fatalf("SaveDrunk failed: %v", err)
	}
	if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{
		Entries: []*pb.CellarEntry{{BeerId: 123, SizeFlOz: 16, DateAdded: 2000}},
	}); err != nil {
		t.Fatalf("SaveCellar failed: %v", err)
	}

	_, err = s.RemoveBeer(ctx, &pb.RemoveBeerRequest{
		Index:     0,
		BeerId:    123,
		SizeFlOz:  16,
		DateAdded: 2000,
	})
	if err != nil {
		t.Fatalf("second RemoveBeer failed: %v", err)
	}

	if got := len(s.untappd.(*TestUntappd).checkins); got != 0 {
		t.Errorf("TestUntappd.checkins len = %d, want 0", got)
	}
	drunkResp, err = s.GetDrunk(ctx, &pb.GetDrunkRequest{Count: 10})
	if err != nil {
		t.Fatalf("GetDrunk failed: %v", err)
	}
	if len(drunkResp.GetDrunk()) != 1 || drunkResp.GetDrunk()[0].GetBeerId() != 999 || drunkResp.GetDrunk()[0].GetDate() != 5555 {
		t.Errorf("GetDrunk changed unexpectedly: %+v", drunkResp.GetDrunk())
	}
	lastDrunk, err := s.db.GetDrunk(ctx, 100)
	if err != nil {
		t.Fatalf("db.GetDrunk failed: %v", err)
	}
	if len(lastDrunk.GetLastCheckins()) != 1 || lastDrunk.GetLastCheckins()[999] != 5555 {
		t.Errorf("db.GetDrunk LastCheckins = %+v, want only {999: 5555}", lastDrunk.GetLastCheckins())
	}
}

func TestRemoveBeer_GoogleTaskRestockThreshold(t *testing.T) {
	setup := func(t *testing.T, count int, googleEnabled bool) (*Server, context.Context) {
		t.Helper()
		ctx, cancel := GetTestContext(context.Background(), time.Minute)
		t.Cleanup(cancel)

		s := getTestServer(ctx)
		s.q = NewTestQueue()

		user := &pb.User{
			Username:           "testuser",
			Auth:               "testuser",
			UserId:             100,
			State:              pb.User_STATE_AUTHORIZED,
			GoogleTasksEnabled: googleEnabled,
		}
		if err := s.db.SaveUser(ctx, user); err != nil {
			t.Fatalf("SaveUser failed: %v", err)
		}
		if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 1, Name: "Light Beer", Abv: 4.0}); err != nil {
			t.Fatalf("SaveBeer failed: %v", err)
		}

		entries := make([]*pb.CellarEntry, 0, count)
		for i := range count {
			entries = append(entries, &pb.CellarEntry{
				BeerId:    1,
				SizeFlOz:  12,
				DateAdded: int64(1000 + i),
			})
		}
		if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{Entries: entries}); err != nil {
			t.Fatalf("SaveCellar failed: %v", err)
		}
		return s, ctx
	}

	t.Run("5 to 4 leaves GoogleTaskActive false", func(t *testing.T) {
		s, ctx := setup(t, 5, true)
		if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000}); err != nil {
			t.Fatalf("RemoveBeer failed: %v", err)
		}
		u, err := s.db.GetUser(ctx, "testuser")
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if u.GetGoogleTaskActive() {
			t.Errorf("GoogleTaskActive = true, want false when 4 weekday beers remain")
		}
		if got := len(s.q.elements); got != 0 {
			t.Errorf("queue len = %d, want 0", got)
		}
	})

	t.Run("4 to 3 activates and 3 to 2 does not duplicate", func(t *testing.T) {
		s, ctx := setup(t, 4, true)
		if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000}); err != nil {
			t.Fatalf("RemoveBeer 4->3 failed: %v", err)
		}
		u, err := s.db.GetUser(ctx, "testuser")
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if !u.GetGoogleTaskActive() {
			t.Errorf("GoogleTaskActive = false, want true when count drops to 3")
		}
		if got := len(s.q.elements); got != 1 {
			t.Fatalf("queue len after 4->3 = %d, want 1", got)
		}

		// Removing again at 3 -> 2 should not enqueue a duplicate task.
		if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1001}); err != nil {
			t.Fatalf("RemoveBeer 3->2 failed: %v", err)
		}
		if got := len(s.q.elements); got != 1 {
			t.Errorf("queue len after 3->2 = %d, want 1 (no duplicate)", got)
		}
	})

	t.Run("1 to 0 activates restock", func(t *testing.T) {
		s, ctx := setup(t, 1, true)
		if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000}); err != nil {
			t.Fatalf("RemoveBeer failed: %v", err)
		}
		u, err := s.db.GetUser(ctx, "testuser")
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if !u.GetGoogleTaskActive() {
			t.Errorf("GoogleTaskActive = false, want true when dropping to 0")
		}
		if got := len(s.q.elements); got != 1 {
			t.Errorf("queue len = %d, want 1", got)
		}
	})

	t.Run("disabled Google Tasks still succeeds", func(t *testing.T) {
		s, ctx := setup(t, 4, false)
		if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000}); err != nil {
			t.Fatalf("RemoveBeer failed: %v", err)
		}
		u, err := s.db.GetUser(ctx, "testuser")
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if !u.GetGoogleTaskActive() {
			t.Errorf("GoogleTaskActive = false, want true")
		}
		s.q.Flush()
	})

	t.Run("SaveUser failure is non-fatal", func(t *testing.T) {
		s, ctx := setup(t, 4, true)
		s.db = &errorInjectingDB{
			Database:    s.db,
			saveUserErr: status.Error(codes.Internal, "save user failed"),
		}
		if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000}); err != nil {
			t.Fatalf("RemoveBeer returned error on SaveUser failure: %v", err)
		}
		cellar, err := s.db.GetCellar(ctx, 100)
		if err != nil {
			t.Fatalf("GetCellar failed: %v", err)
		}
		if len(cellar.GetEntries()) != 3 {
			t.Errorf("len(Entries) = %d, want 3", len(cellar.GetEntries()))
		}
	})

	t.Run("SaveCellar failure returns error and skips restock", func(t *testing.T) {
		s, ctx := setup(t, 4, true)
		s.db = &errorInjectingDB{
			Database:      s.db,
			saveCellarErr: status.Error(codes.Internal, "save cellar failed"),
		}
		_, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000})
		if status.Code(err) != codes.Internal {
			t.Fatalf("RemoveBeer err = %v (code %v), want Internal on SaveCellar failure", err, status.Code(err))
		}
		u, err := s.db.GetUser(ctx, "testuser")
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if u.GetGoogleTaskActive() {
			t.Errorf("GoogleTaskActive = true, want false when SaveCellar fails")
		}
		if got := len(s.q.elements); got != 0 {
			t.Errorf("queue len = %d, want 0 when SaveCellar fails", got)
		}
	})
}

func TestRemoveBeer_ValidationAndStaleIndex(t *testing.T) {
	ctx, cancel := GetTestContext(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)
	s.q = NewTestQueue()

	// Empty cellar (NotFound in DB) -> OutOfRange
	if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000}); status.Code(err) != codes.OutOfRange {
		t.Errorf("empty cellar (NotFound) code = %v, want OutOfRange", status.Code(err))
	}

	// Validation errors
	validationCases := []struct {
		name string
		req  *pb.RemoveBeerRequest
	}{
		{"negative index", &pb.RemoveBeerRequest{Index: -1, BeerId: 1}},
		{"zero beer_id", &pb.RemoveBeerRequest{Index: 0, BeerId: 0}},
		{"negative beer_id", &pb.RemoveBeerRequest{Index: 0, BeerId: -5}},
	}
	for _, tc := range validationCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.RemoveBeer(ctx, tc.req); status.Code(err) != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", status.Code(err))
			}
		})
	}

	// Empty cellar (saved with 0 entries) -> OutOfRange
	if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{}); err != nil {
		t.Fatalf("SaveCellar empty failed: %v", err)
	}
	if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1, SizeFlOz: 12, DateAdded: 1000}); status.Code(err) != codes.OutOfRange {
		t.Errorf("empty saved cellar code = %v, want OutOfRange", status.Code(err))
	}

	// Seed 1 entry
	entry := &pb.CellarEntry{BeerId: 100, SizeFlOz: 12, DateAdded: 1000}
	if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{Entries: []*pb.CellarEntry{entry}}); err != nil {
		t.Fatalf("SaveCellar failed: %v", err)
	}

	// Out of bounds index -> OutOfRange
	if _, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{Index: 1, BeerId: 100, SizeFlOz: 12, DateAdded: 1000}); status.Code(err) != codes.OutOfRange {
		t.Errorf("out-of-range index code = %v, want OutOfRange", status.Code(err))
	}

	// Stale/mismatched entry checks -> FailedPrecondition with cellar unmodified
	mismatchCases := []struct {
		name string
		req  *pb.RemoveBeerRequest
	}{
		{"mismatched beer_id", &pb.RemoveBeerRequest{Index: 0, BeerId: 999, SizeFlOz: 12, DateAdded: 1000}},
		{"mismatched size_fl_oz", &pb.RemoveBeerRequest{Index: 0, BeerId: 100, SizeFlOz: 16, DateAdded: 1000}},
		{"mismatched date_added", &pb.RemoveBeerRequest{Index: 0, BeerId: 100, SizeFlOz: 12, DateAdded: 9999}},
	}
	for _, tc := range mismatchCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.RemoveBeer(ctx, tc.req); status.Code(err) != codes.FailedPrecondition {
				t.Errorf("code = %v, want FailedPrecondition", status.Code(err))
			}
			c, err := s.db.GetCellar(ctx, 100)
			if err != nil {
				t.Fatalf("GetCellar failed: %v", err)
			}
			if len(c.GetEntries()) != 1 || c.GetEntries()[0].GetBeerId() != 100 || c.GetEntries()[0].GetSizeFlOz() != 12 || c.GetEntries()[0].GetDateAdded() != 1000 {
				t.Errorf("cellar modified on FailedPrecondition: %+v", c.GetEntries())
			}
		})
	}
}

func TestRemoveBeer_AuthAndIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s := getTestServer(ctx)
	s.q = NewTestQueue()

	// Missing auth-token -> NotFound
	if _, err := s.RemoveBeer(context.Background(), &pb.RemoveBeerRequest{Index: 0, BeerId: 1}); status.Code(err) != codes.NotFound {
		t.Errorf("missing token code = %v, want NotFound", status.Code(err))
	}

	// Unknown auth-token -> NotFound
	unknownCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("auth-token", "does-not-exist"))
	if _, err := s.RemoveBeer(unknownCtx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1}); status.Code(err) != codes.NotFound {
		t.Errorf("unknown token code = %v, want NotFound", status.Code(err))
	}

	// Unauthorized user (STATE_LOGGING_IN) -> PermissionDenied
	if err := s.db.SaveUser(ctx, &pb.User{
		Username: "pending",
		Auth:     "pending-token",
		UserId:   150,
		State:    pb.User_STATE_LOGGING_IN,
	}); err != nil {
		t.Fatalf("SaveUser pending failed: %v", err)
	}
	pendingCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("auth-token", "pending-token"))
	if _, err := s.RemoveBeer(pendingCtx, &pb.RemoveBeerRequest{Index: 0, BeerId: 1}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("unauthorized user code = %v, want PermissionDenied", status.Code(err))
	}

	// Isolation between two authorized users
	if err := s.db.SaveUser(ctx, &pb.User{
		Username: "user2",
		Auth:     "user2-token",
		UserId:   200,
		State:    pb.User_STATE_AUTHORIZED,
	}); err != nil {
		t.Fatalf("SaveUser user2 failed: %v", err)
	}
	if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{
		Entries: []*pb.CellarEntry{{BeerId: 100, SizeFlOz: 12, DateAdded: 1000}},
	}); err != nil {
		t.Fatalf("SaveCellar user1 failed: %v", err)
	}

	user2Ctx := metadata.NewIncomingContext(ctx, metadata.Pairs("auth-token", "user2-token"))
	if _, err := s.RemoveBeer(user2Ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 100, SizeFlOz: 12, DateAdded: 1000}); status.Code(err) != codes.OutOfRange {
		t.Errorf("user2 remove on empty cellar code = %v, want OutOfRange", status.Code(err))
	}

	// Give user2 an identical-looking entry and remove it; user1's cellar must stay intact.
	if err := s.db.SaveCellar(ctx, 200, &pb.Cellar{
		Entries: []*pb.CellarEntry{{BeerId: 100, SizeFlOz: 12, DateAdded: 1000}},
	}); err != nil {
		t.Fatalf("SaveCellar user2 failed: %v", err)
	}
	if _, err := s.RemoveBeer(user2Ctx, &pb.RemoveBeerRequest{Index: 0, BeerId: 100, SizeFlOz: 12, DateAdded: 1000}); err != nil {
		t.Fatalf("user2 RemoveBeer failed: %v", err)
	}

	u1Cellar, err := s.db.GetCellar(ctx, 100)
	if err != nil {
		t.Fatalf("GetCellar user1 failed: %v", err)
	}
	if len(u1Cellar.GetEntries()) != 1 {
		t.Errorf("user1 cellar entries = %d, want 1", len(u1Cellar.GetEntries()))
	}
	u2Cellar, err := s.db.GetCellar(ctx, 200)
	if err != nil {
		t.Fatalf("GetCellar user2 failed: %v", err)
	}
	if len(u2Cellar.GetEntries()) != 0 {
		t.Errorf("user2 cellar entries = %d, want 0", len(u2Cellar.GetEntries()))
	}
}

func TestRemoveBeer_ConcurrentSafety(t *testing.T) {
	t.Run("concurrent RemoveBeer AddBeer and RefreshUser", func(t *testing.T) {
		ctx, cancel := GetTestContext(context.Background(), time.Minute)
		defer cancel()

		s := getTestServer(ctx)
		if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 100, Name: "Beer 100", Brewery: "Brewery", Abv: 5.0}); err != nil {
			t.Fatalf("SaveBeer 100 failed: %v", err)
		}
		if err := s.db.SaveBeer(ctx, &pb.Beer{Id: 101, Name: "Beer 101", Brewery: "Brewery", Abv: 5.0}); err != nil {
			t.Fatalf("SaveBeer 101 failed: %v", err)
		}

		initial := make([]*pb.CellarEntry, 0, 10)
		for i := range 10 {
			initial = append(initial, &pb.CellarEntry{
				BeerId:    100,
				SizeFlOz:  12,
				DateAdded: int64(1000 + i),
			})
		}
		if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{Entries: initial}); err != nil {
			t.Fatalf("SaveCellar failed: %v", err)
		}

		s.untappd.(*TestUntappd).checkins = []*pb.Checkin{
			{CheckinId: 1, BeerId: 101, Date: time.Now().Unix(), Rating: 4},
		}

		var wg sync.WaitGroup
		errCh := make(chan error, 100)

		// AddBeer workers
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					if _, err := s.AddBeer(ctx, &pb.AddBeerRequest{BeerId: 100, SizeFlOz: 12, Quantity: 1}); err != nil {
						errCh <- err
						return
					}
				}
			}()
		}

		// RemoveBeer workers
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					c, err := s.GetCellar(ctx, &pb.GetCellarRequest{})
					if err != nil {
						errCh <- err
						return
					}
					if len(c.GetEntries()) == 0 {
						continue
					}
					e := c.GetEntries()[0]
					_, err = s.RemoveBeer(ctx, &pb.RemoveBeerRequest{
						Index:     0,
						BeerId:    e.GetBeerId(),
						SizeFlOz:  e.GetSizeFlOz(),
						DateAdded: e.GetDateAdded(),
					})
					if err != nil {
						code := status.Code(err)
						if code != codes.OutOfRange && code != codes.FailedPrecondition {
							errCh <- err
							return
						}
					}
				}
			}()
		}

		// RefreshUser workers
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
					if _, err := s.RefreshUser(ctx, &pb.RefreshUserRequest{Username: "testuser"}); err != nil {
						errCh <- err
						return
					}
				}
			}()
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			t.Fatalf("concurrent operation failed unexpectedly: %v", err)
		}

		s.q.Flush()

		resp, err := s.GetCellar(ctx, &pb.GetCellarRequest{})
		if err != nil {
			t.Fatalf("final GetCellar failed: %v", err)
		}
		if len(resp.GetBeers()) != len(resp.GetEntries()) {
			t.Errorf("len(Beers)=%d != len(Entries)=%d", len(resp.GetBeers()), len(resp.GetEntries()))
		}
	})

	t.Run("RefreshUser does not hold cellarMu during GetCheckins", func(t *testing.T) {
		ctx, cancel := GetTestContext(context.Background(), time.Minute)
		defer cancel()

		s := getTestServer(ctx)
		s.q = NewTestQueue()
		ut := &blockingCheckinsUntappd{
			TestUntappd: &TestUntappd{},
			started:     make(chan struct{}),
			release:     make(chan struct{}),
		}
		s.untappd = ut

		if err := s.db.SaveCellar(ctx, 100, &pb.Cellar{
			Entries: []*pb.CellarEntry{{BeerId: 100, SizeFlOz: 12, DateAdded: 1000}},
		}); err != nil {
			t.Fatalf("SaveCellar failed: %v", err)
		}

		refreshDone := make(chan error, 1)
		go func() {
			_, err := s.RefreshUser(ctx, &pb.RefreshUserRequest{Username: "testuser"})
			refreshDone <- err
		}()

		select {
		case <-ut.started:
		case <-time.After(2 * time.Second):
			close(ut.release)
			t.Fatal("timed out waiting for RefreshUser to call GetCheckins")
		}

		removeDone := make(chan error, 1)
		go func() {
			_, err := s.RemoveBeer(ctx, &pb.RemoveBeerRequest{
				Index:     0,
				BeerId:    100,
				SizeFlOz:  12,
				DateAdded: 1000,
			})
			removeDone <- err
		}()

		select {
		case err := <-removeDone:
			if err != nil {
				close(ut.release)
				t.Fatalf("RemoveBeer failed while RefreshUser was in GetCheckins: %v", err)
			}
		case <-time.After(2 * time.Second):
			close(ut.release)
			t.Fatal("RemoveBeer blocked while RefreshUser was in GetCheckins (cellarMu held across HTTP call)")
		}

		close(ut.release)
		if err := <-refreshDone; err != nil {
			t.Fatalf("RefreshUser failed: %v", err)
		}
	})
}
