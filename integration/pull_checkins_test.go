//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"testing"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPullCheckins(t *testing.T) {
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

	iconn, err := grpc.NewClient(fmt.Sprintf(":%v", i.mp), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Unable to connect to server: %v", err)
	}
	iclient := pb.NewBeerKellerAdminClient(iconn)

	ctx, cancel = GetTestContext(context.Background(), time.Minute*5)
	defer cancel()

	_, err = client.AddBeer(ctx, &pb.AddBeerRequest{
		BeerId:   16630, // Sierra Nevada Celebration Ale
		Quantity: 12,
		SizeFlOz: 12})
	if err != nil {
		t.Fatalf("Unable to add beer: %v", err)
	}

	time.Sleep(70 * time.Second)

	cellar, err := client.GetCellar(ctx, &pb.GetCellarRequest{})
	if err != nil {
		t.Fatalf("Unable to retrieve cellar: %v", err)
	}

	if len(cellar.GetBeers()) != 12 {
		t.Fatalf("Cellar only contains %v entries, should have 12", len(cellar.GetBeers()))
	}

	// Let's drink one of these beers
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%v/checkin/16630", i.ump), nil)
	if err != nil {
		t.Fatalf("Unable to checkin beer: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unable to checking beer: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Bad read: %v", err)
	}
	log.Printf("Checkin response: %v (%v) -> %v", resp, resp.StatusCode, string(body))

	// And we need to trigger a checkin pull
	_, err = iclient.RefreshUser(ctx, &pb.RefreshUserRequest{Username: "testuser"})
	if err != nil {
		t.Fatalf("Unable to refresh user: %v", err)
	}

	// Two things should happen
	// First, we should have an entry in our drunk map
	drunk, err := client.GetDrunk(ctx, &pb.GetDrunkRequest{})
	if err != nil {
		t.Fatalf("Unable to get drunks: %v", err)
	}

	if time.Since(time.Unix(drunk.GetDrunk()[16630], 0)) < time.Hour {
		t.Fatalf("The drunk map is broken: %v", drunk)
	}

	// Second, we should have a smaller cellar
	cellar, err = client.GetCellar(ctx, &pb.GetCellarRequest{})
	if err != nil {
		t.Fatalf("Unable to retrieve cellar: %v", err)
	}

	if len(cellar.GetBeers()) != 11 {
		t.Fatalf("Cellar only contains %v entries, should have 11 (we drunk one)", len(cellar.GetBeers()))
	}

}
