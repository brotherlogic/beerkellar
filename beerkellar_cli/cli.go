package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/browser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/prototext"
	pb "github.com/brotherlogic/beerkellar/proto"

	tea "github.com/charmbracelet/bubbletea"
)

const weekdayBeerUnitsLimit = 3.5

func runTuiTestLoop(model tea.Model) {
	if tui, ok := model.(tuiModel); ok {
		summaryMsg := tui.fetchCellarSummary()()
		model, _ = model.Update(summaryMsg)
		statusMsg := tui.checkInitialStatus()()
		model, _ = model.Update(statusMsg)
	}

	fmt.Println(model.View())

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "exit" || line == "quit" {
			break
		}

		var cmd tea.Cmd
		for _, char := range line {
			model, cmd = model.Update(tea.KeyMsg{Runes: []rune{char}})
			if cmd != nil {
				msg := cmd()
				model, _ = model.Update(msg)
			}
		}
		model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{'\r'}})
		if cmd != nil {
			msg := cmd()
			model, _ = model.Update(msg)
		}

		fmt.Println(model.View())
	}
}

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
	addr := "beerkellar-grpc.brotherlogic-backend.com:80"
	if envAddr := os.Getenv("BEERKELLAR_SERVER_ADDR"); envAddr != "" {
		addr = envAddr
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect: %v", err)
	}

	client := pb.NewBeerKellerClient(conn)

	ctx, cancel, err := buildContext()
	if err != nil {
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute*5)
	}
	defer cancel()

	googleClient := pb.NewBeerKellerGoogleClient(conn)

	if len(os.Args) < 2 {
		model := initialModel(client, googleClient)
		if os.Getenv("TUI_TEST_MODE") == "true" {
			runTuiTestLoop(model)
			return
		}
		p := tea.NewProgram(model)
		if _, err := p.Run(); err != nil {
			log.Fatalf("Error running TUI: %v", err)
		}
		return
	}

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
		var weekday, nonWeekday int
		for i, beer := range cellar.GetBeers() {
			log.Printf("%v. %v - %v (%v) [%v] [%.2f units]", i+1, beer.GetBrewery(), beer.GetName(), beer.GetAbv(), beer.GetId(), beer.GetUnits())
			if beer.GetUnits() < weekdayBeerUnitsLimit {
				weekday++
			} else {
				nonWeekday++
			}
		}
		log.Printf("Summary: %v weekday beers, %v non-weekday beers", weekday, nonWeekday)
	case "pull":
		pullSet := flag.NewFlagSet("pull_beer", flag.ExitOnError)
		weekday := pullSet.Bool("weekday", true, "Whether it's a weekday (limit to 3.5 units)")
		if err := pullSet.Parse(os.Args[2:]); err == nil {
			req := &pb.GetBeerRequest{
				NoRepeat: true,
				Requirements: []*pb.BeerRequirement{
					{
						Strategy: pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK,
					},
				},
			}
			log.Printf("Weekday flag: %v", *weekday)
			if *weekday {
				req.Requirements[0].MaxUnits = weekdayBeerUnitsLimit
			}
			log.Printf("Requirement 0: %+v", req.Requirements[0])

			res, err := client.GetBeer(ctx, req)
			if err != nil {
				log.Fatalf("Error pulling beer: %v", err)
			}
			if len(res.GetBeers()) > 0 {
				beer := res.GetBeers()[0]
				log.Printf("Pulled beer: %v - %v (%v%% ABV) [%v] [%.2f units]", beer.GetBrewery(), beer.GetName(), beer.GetAbv(), beer.GetId(), beer.GetUnits())
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
	case "drink":
		drinkSet := flag.NewFlagSet("drink_beer", flag.ExitOnError)
		bid := drinkSet.Int64("id", -1, "The id of the beer to drink")
		if err := drinkSet.Parse(os.Args[2:]); err == nil {
			if *bid <= 0 {
				log.Fatalf("You must specify a beer id to drink (-id)")
			}

			_, err := client.DrinkBeer(ctx, &pb.DrinkBeerRequest{
				BeerId: *bid,
			})
			if err != nil {
				log.Fatalf("Error drinking beer: %v", err)
			}
			log.Printf("Beer %v recorded as drunk.", *bid)
		}
	case "google":
		googleClient := pb.NewBeerKellerGoogleClient(conn)
		if len(os.Args) < 3 {
			log.Fatalf("Usage: google [login|tasks]")
		}
		switch os.Args[2] {
		case "login":
			res, err := googleClient.GetGoogleLogin(ctx, &pb.GetGoogleLoginRequest{})
			if err != nil {
				log.Fatalf("Unable to get google login url: %v", err)
			}
			log.Printf("Opening Google Login URL. After completing, the window will say 'Google Account Linked Successfully!'.")
			err = browser.OpenURL(res.GetUrl())
			if err != nil {
				log.Fatalf("unable to open URL: %v", err)
			}
		case "tasks":
			if len(os.Args) < 4 {
				log.Fatalf("Usage: google tasks [on|off]")
			}
			enable := os.Args[3] == "on"
			_, err := googleClient.ToggleGoogleTasks(ctx, &pb.ToggleGoogleTasksRequest{Enable: enable})
			if err != nil {
				log.Fatalf("Unable to toggle tasks: %v", err)
			}
			log.Printf("Google Tasks feature toggled: %v", enable)
		}
	}
}
