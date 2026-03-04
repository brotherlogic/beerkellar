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
)

func TestGetBeer_NoMultiples(t *testing.T) {
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
		SizeFlOz: 12,
		Quantity: 2})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	// Add a beer
	_, err = client.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   6284, // Sierra Nevada Pale Ale
		SizeFlOz: 2000,
		Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	time.Sleep(time.Second)

	beer, err := client.GetBeer(ctx, &pb.GetBeerRequest{
		Requirements: []*pb.BeerRequirement{
			{MaxUnits: 20}, {MaxUnits: 20}}})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}

	// We should have got two beers - both pale ales
	if len(beer.GetBeers()) != 2 {
		t.Fatalf("Did not get two beers: %v", beer)
	}
	for _, b := range beer.GetBeers() {
		if b.GetId() != 16630 {
			t.Fatalf("Did not get two celebrations: %v", b)
		}
	}

	beer, err = client.GetBeer(ctx, &pb.GetBeerRequest{
		NoRepeat: true,
		Requirements: []*pb.BeerRequirement{
			{MaxUnits: 20}, {MaxUnits: 20}}})

	// should have only got one beer: celebration
	if len(beer.GetBeers()) != 1 || beer.GetBeers()[0].GetId() != 16630 {
		t.Errorf("Problem in the no repeat stage: wrong beer or too many: %v", beer)
	}

}
