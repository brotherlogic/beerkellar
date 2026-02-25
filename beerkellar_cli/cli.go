package main

import (
	"context"
	"log"
	"os"

	"github.com/pkg/browser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func main() {
	conn, err := grpc.NewClient("beerkellar.brotherlogic-backend.com:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect: %v", err)
	}

	client := pb.NewBeerKellerClient(conn)

	switch os.Args[1] {
	case "login":
		url, err := client.GetLogin(context.Background(), &pb.GetLoginRequest{})
		if err != nil {
			log.Fatalf("Unable to get login: %v", err)
		}
		err = browser.OpenURL(url.GetUrl())
		if err != nil {
			log.Fatalf("unable to open URL: %v", err)
		}
	}
}
