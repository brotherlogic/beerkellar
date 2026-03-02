package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/browser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func buildContext() (context.Context, context.CancelFunc, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}

	text, err := ioutil.ReadFile(fmt.Sprintf("%v/.beerkellar", dirname))
	if err != nil {
		return nil, nil, err
	}

	user := &pb.GetAuthTokenResponse{}
	err = prototext.Unmarshal(text, user)
	if err != nil {
		return nil, nil, err
	}

	mContext := metadata.AppendToOutgoingContext(context.Background(), "auth-token", user.GetCode())
	ctx, cancel := context.WithTimeout(mContext, time.Minute*30)
	return ctx, cancel, nil
}

func main() {
	conn, err := grpc.NewClient("beerkellar-grpc.brotherlogic-backend.com:80", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect: %v", err)
	}

	client := pb.NewBeerKellerClient(conn)

	ctx, cancel, err := buildContext()
	if err != nil {
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute*5)
	}
	defer cancel()

	switch os.Args[1] {
	case "add":
		addSet := flag.NewFlagSet("add_beer", flag.ExitOnError)
		bid := addSet.Int64("id", -1, "The id of the beer to add")
		quantity := addSet.Int64("quantity", -1, "The number of beers to add")

		if err := addSet.Parse(os.Args[2:]); err == nil {
			res, err := client.AddBeer(ctx, &pb.AddBeerRequest{
				BeerId:   *bid,
				Quantity: int32(*quantity),
			})
			if err != nil {
				log.Fatalf("Error adding beers: %v", err)
			}
			log.Printf("Beers added: %v", res)
		}
	case "login":
		url, err := client.GetLogin(context.Background(), &pb.GetLoginRequest{})
		if err != nil {
			log.Fatalf("Unable to get login: %v", err)
		}
		err = browser.OpenURL(url.GetUrl())
		if err != nil {
			log.Fatalf("unable to open URL: %v", err)
		}

		t := time.Now()
		for time.Since(t) < time.Minute {
			time.Sleep(time.Second * 5)
			auth, err := client.GetAuthToken(ctx, &pb.GetAuthTokenRequest{
				Code: url.GetCode(),
			})

			if err != nil {
				log.Printf("Bad auth: %v", err)
				continue
			}
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("Unable to get home dir")
			}
			f, err := os.OpenFile(fmt.Sprintf("%v/.beerkellar", dirname), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				log.Fatalf("unable to open file")
			}
			defer f.Close()
			if proto.MarshalText(f, auth) != nil {
				log.Fatalf("Unable to save token")
			}
			break
		}
	}
}
