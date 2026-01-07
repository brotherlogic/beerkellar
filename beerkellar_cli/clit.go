package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func main() {
	conn, err := grpc.Dial("beer.brotherlogic-backend.com:8080")
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
		fmt.Println(url)
	}
}
