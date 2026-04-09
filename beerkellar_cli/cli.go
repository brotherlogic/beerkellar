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

		size := addSet.Int64("size", -1, "The size of the beer in fluid ounces")
		if err := addSet.Parse(os.Args[2:]); err == nil {
			if *size <= 0 {
				log.Fatalf("You must specify a size for the beer (-size)")
			}

			res, err := client.AddBeer(ctx, &pb.AddBeerRequest{
				BeerId:   *bid,
				Quantity: int32(*quantity),
				SizeFlOz: int32(*size),
			})
			if err != nil {
				log.Fatalf("Error adding beers: %v", err)
			}
			log.Printf("Beers added: %v", res)
		}
	case "cellar":
		cellar, err := client.GetCellar(ctx, &pb.GetCellarRequest{})
		if err != nil {
			log.Fatalf("Unable to get cellar: %v", err)
		}
		log.Printf("User: %v (State: %v)", cellar.GetUsername(), cellar.GetState())
		for i, beer := range cellar.GetBeers() {
			log.Printf("%v. %v - %v (%v) [%v]", i+1, beer.GetBrewery(), beer.GetName(), beer.GetAbv(), beer.GetId())
		}
	case "pull":
		pullSet := flag.NewFlagSet("pull_beer", flag.ExitOnError)
		weekday := pullSet.Bool("weekday", false, "Whether it's a weekday (limit to 2.5 units)")
		if err := pullSet.Parse(os.Args[2:]); err == nil {
			req := &pb.GetBeerRequest{
				NoRepeat: true,
				Requirements: []*pb.BeerRequirement{
					{
						Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK,
					},
				},
			}
			if *weekday {
				req.Requirements[0].MaxUnits = 2.5
			}

			res, err := client.GetBeer(ctx, req)
			if err != nil {
				log.Fatalf("Error pulling beer: %v", err)
			}
			if len(res.GetBeers()) > 0 {
				beer := res.GetBeers()[0]
				log.Printf("Pulled beer: %v - %v (%v%% ABV) [%v]", beer.GetBrewery(), beer.GetName(), beer.GetAbv(), beer.GetId())
			} else {
				log.Printf("No beers found matching requirements")
			}
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
	case "drunk":
		drunkSet := flag.NewFlagSet("drunk_beers", flag.ExitOnError)
		count := drunkSet.Int("count", 10, "The number of drunk beers to return")
		if err := drunkSet.Parse(os.Args[2:]); err == nil {
			res, err := client.GetDrunk(ctx, &pb.GetDrunkRequest{
				Count: int32(*count),
			})
			if err != nil {
				log.Fatalf("Error getting drunk beers: %v", err)
			}
			for _, beer := range res.GetDrunk() {
				dateStr := time.Unix(beer.GetDate(), 0).Format("2006-01-02")
				if beer.GetName() == "" {
					log.Printf("%v Unknown - Unknown [%v]", dateStr, beer.GetBeerId())
				} else {
					log.Printf("%v %v - %v (%.2f units)", dateStr, beer.GetBrewery(), beer.GetName(), beer.GetUnits())
				}
			}
		}
	}
}
