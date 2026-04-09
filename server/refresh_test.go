package server

import (
	"context"
	"testing"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func TestRunRefresh(t *testing.T) {
	ctx := context.Background()
	s := getTestServer(ctx)

	err := s.db.SaveUser(ctx, &pb.User{
		Username:     "user1",
		Auth:         "auth1",
		UserId:       101,
		LastFeedPull: 0,
		State:        pb.User_STATE_AUTHORIZED,
	})
	if err != nil {
		t.Fatalf("Unable to save user: %v", err)
	}

	// Add a user who doesn't need refresh (LastFeedPull = now)
	now := time.Now().Unix()
	err = s.db.SaveUser(ctx, &pb.User{
		Username:     "user2",
		Auth:         "auth2",
		UserId:       102,
		LastFeedPull: now,
		State:        pb.User_STATE_AUTHORIZED,
	})
	if err != nil {
		t.Fatalf("Unable to save user: %v", err)
	}

	// Add an unauthorized user
	err = s.db.SaveUser(ctx, &pb.User{
		Username:     "user3",
		Auth:         "auth3",
		UserId:       103,
		LastFeedPull: 0,
		State:        pb.User_STATE_LOGGING_IN,
	})
	if err != nil {
		t.Fatalf("Unable to save user: %v", err)
	}

	// Mock a checkin so that LastFeedPull advances
	s.untappd.(*TestUntappd).checkins = []*pb.Checkin{
		{CheckinId: 1001, BeerId: 1, Date: time.Now().Unix()},
	}

	s.runRefresh(ctx)

	// Check if user1 was refreshed (LastFeedPull should be updated)
	u1, err := s.db.GetUserByName(ctx, "user1")
	if err != nil {
		t.Fatalf("Unable to get user1: %v", err)
	}
	if u1.GetLastFeedPull() == 0 {
		t.Errorf("User1 was not refreshed")
	}

	// Check if user2 was NOT refreshed (LastFeedPull should be the same)
	u2, err := s.db.GetUserByName(ctx, "user2")
	if err != nil {
		t.Fatalf("Unable to get user2: %v", err)
	}
	if u2.GetLastFeedPull() != now {
		t.Errorf("User2 was refreshed when it shouldn't have been: %v vs %v", u2.GetLastFeedPull(), now)
	}

	// Check if user3 was NOT refreshed (LastFeedPull should be the same)
	u3, err := s.db.GetUserByName(ctx, "user3")
	if err != nil {
		t.Fatalf("Unable to get user3: %v", err)
	}
	if u3.GetLastFeedPull() != 0 {
		t.Errorf("Unauthorized user was refreshed: %v", u3.GetLastFeedPull())
	}
}
