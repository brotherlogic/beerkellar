//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func GetTestContext(ctx context.Context, deadline time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, deadline)
	ctx = metadata.AppendToOutgoingContext(context.Background(),
		"auth-token",
		"testuser")
	return ctx, cancel
}

func TestGetBeer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	i, err := runTestServer(ctx, t)
	if err != nil {
		t.Fatalf("Unable to bring up server: %v", err)
	}
	defer i.teardownContainer(t)

	// Let's try logging in
	conn, err := grpc.NewClient(fmt.Sprintf(":%v", i.mp), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Unable to connect to server: %v", err)
	}
	client := pb.NewBeerKellerClient(conn)

	ctx, cancel = GetTestContext(context.Background(), time.Minute*5)
	defer cancel()

	// Add a beer
	_, err = client.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   16630, // Sierra Nevada Celebration Ale
		Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	// Add a beer
	_, err = client.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   6284, // Sierra Nevada Celebration Ale
		Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	time.Sleep(time.Second)

	counts := make(map[int64]int)
	for i := 0; i < 100; i++ {
		beer, err := client.GetBeer(ctx, &pb.GetBeerRequest{Requirements: []*pb.BeerRequirement{{}}})
		if err != nil {
			t.Fatalf("Unable to get beer: %v", err)
		}
		for _, beer := range beer.GetBeers() {
			counts[beer.GetId()]++
		}
	}

	// We should have picked both beers
	if len(counts) != 2 {
		t.Errorf("Expected 2 beers, got %v", counts)
	}

}
