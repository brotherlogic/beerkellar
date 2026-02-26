package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/browser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func main() {
	conn, err := grpc.NewClient("beerkellar-grpc.brotherlogic-backend.com:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect: %v", err)
	}

	client := pb.NewBeerKellerClient(conn)

	switch os.Args[1] {
	case "login":
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
		defer cancel()

		url, err := client.GetLogin(ctx, &pb.GetLoginRequest{})
		if err != nil {
			log.Fatalf("Unable to get login: %v", err)
		}
		err = browser.OpenURL(url.GetUrl())
		if err != nil {
			log.Fatalf("unable to open URL: %v", err)
		}

		t := time.Now()
		for time.Since(t) < time.Minute*5 {
			time.Sleep(time.Second * 5)

			auth, err := client.GetAuthToken(ctx, &pb.GetAuthTokenRequest{
				Code: url.GetCode(),
			})
			if err != nil {
				if status.Code(err) != codes.DataLoss {
					log.Fatalf("Bad return: %v", err)
				} else {
					continue
				}
			}

			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("No dir")
			}

			f, err := os.OpenFile(fmt.Sprintf("%v/.beerkellar", dirname), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				log.Fatalf("Cannot create: %v", err)
			}
			defer f.Close()
			err = proto.MarshalText(f, auth)
			if err != nil {
				log.Fatalf("Unable to marshal: %v", err)
			}
		}
	}
}
