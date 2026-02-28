//go:build skip

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

func getTestContext(ctx context.Context, deadline time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, deadline)
	ctx = metadata.AppendToOutgoingContext(context.Background(),
		"auth-token",
		"testuser")
	return ctx, cancel
}

func TestAddBeer(t *testing.T) {
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

	ctx, cancel = getTestContext(context.Background(), time.Minute*5)
	defer cancel()

	// Add a beer
	_, err = client.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   6284, // Sierra Nevada Pale Ale
		Quantity: 12})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	foundAbv := false
	ti := time.Now()
	for !foundAbv && time.Since(ti) < time.Minute*5 {
		cellar, err := client.GetCellar(ctx, &pb.GetCellarRequest{})
		if err != nil {
			t.Fatalf("Unable to retrieve cellar: %v", err)
		}

		if len(cellar.GetBeers()) != 12 {
			t.Fatalf("Cellar only contains %v entries, should have 12", len(cellar.GetBeers()))
		}

		if cellar.GetBeers()[0].GetAbv() == 5.6 {
			foundAbv = true
		}
	}

	if !foundAbv {
		t.Errorf("Cellar was not refreshed once beer added")
	}

}
