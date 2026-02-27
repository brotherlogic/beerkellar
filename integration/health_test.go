//go:build skip

package integration

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func DontTestHealth(t *testing.T) {
	ctx := context.Background()
	i, err := runTestServer(ctx, t)
	if err != nil {
		t.Fatalf("Unable to bring up server: %v", err)
	}

	log.Printf("Running: %v", i)
	defer i.teardownContainer(t)

	// Send a basic health check to the server via grpc
	conn, err := grpc.NewClient(fmt.Sprintf(":%v", i.mp), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("%v", err)
	}
	client := pb.NewBeerKellerClient(conn)
	_, err = client.Healthy(ctx, &pb.HealthyRequest{})
	if err != nil {
		t.Errorf("Unable to determine health: %v", err)
	}
	time.Sleep(time.Second * 30)

}
