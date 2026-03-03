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

func TestGetBeerWithRequirements_Random(t *testing.T) {
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
		SizeFlOz: 20000,
		Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	// Add a beer
	_, err = client.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   6284, // Sierra Nevada Pale Ale
		SizeFlOz: 16,
		Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	beer, err := client.GetBeer(ctx, &pb.GetBeerRequest{Requirement: &pb.BeerRequirement{
		Strategy: pb.BeerRequirement_STRATEGY_RANDOM,
		MaxUnits: 5,
	}})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}

	if beer.GetBeer().GetId() != 6284 {
		t.Errorf("Expected beer %v, got %v", 6284, beer.GetBeer())
	}
}

func TestGetBeerWithRequirements_Oldest(t *testing.T) {
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
		SizeFlOz: 16,
		Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	// Add a beer
	_, err = client.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   6284, // Sierra Nevada Pale Ale
		SizeFlOz: 16,
		Quantity: 1})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	beer, err := client.GetBeer(ctx, &pb.GetBeerRequest{Requirement: &pb.BeerRequirement{
		Strategy: pb.BeerRequirement_STRATEGY_OLDEST,
		MaxUnits: 5,
	}})
	if err != nil {
		t.Fatalf("Unable to get beer: %v", err)
	}

	if beer.GetBeer().GetId() != 16630 {
		t.Errorf("Expected beer %v, got %v", 16630, beer.GetBeer())
	}
}
