package integration

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestLogin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	i, err := runTestServer(ctx)
	if err != nil {
		t.Fatalf("Unable to bring up server: %v", err)
	}

	log.Printf("Running: %v", i)
	defer i.teardownContainer(t)

	// Let's try logging in
	conn, err := grpc.NewClient(fmt.Sprintf(":%v", i.mp), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Unable to connect to server: %v", err)
	}
	client := pb.NewBeerKellerClient(conn)

	lurl, err := client.GetLogin(ctx, &pb.GetLoginRequest{})
	if err != nil {
		t.Fatalf("Unable to get loging url: %v", err)
	}

	log.Printf("URL: %v, Code: %v", lurl.GetUrl(), lurl.GetCode())

	// Let's run the login URL
	resp, err := http.DefaultClient.Get(lurl.GetUrl())
	if err != nil {
		log.Fatalf("Unable to ping login: %v -> %v", lurl, resp)
	}

	// And then get the authenticated user
	_, err = client.GetAuthToken(ctx, &pb.GetAuthTokenRequest{Code: lurl.GetCode()})
	if err != nil {
		log.Fatalf("Bad request for user: %v", err)
	}

}
