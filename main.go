package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beerkellar/proto"
	"github.com/brotherlogic/beerkellar/server"
)

var (
	port         = flag.Int("port", 8080, "Server port for grpc traffic")
	metricsPort  = flag.Int("metrics_port", 8081, "Metrics port")
	callbackPort = flag.Int("callback_port", 8082, "Callback port")

	baseUntappdAPI  = flag.String("untappd_url", "https://api.untappd.com", "Base URL for reaching untappd API")
	baseUntappdAuth = flag.String("untappd_auth", "https://untappd.com", "Base URL for doing auth")
	testDb          = flag.Bool("test_db", false, "If true, use a test db")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	var db server.Database
	if *testDb {
		db = server.NewTestDatabase(ctx)
	} else {
		db = server.NewDatabase(ctx)
	}
	cancel()

	s := server.NewServer(
		os.Getenv("client_id"),
		os.Getenv("client_secret"),
		os.Getenv("redirect_url"),
		db)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%v", *metricsPort), nil)
		log.Fatalf("Beerkellar is unable to serve metrics: %v", err)
	}()

	http.Handle("/", http.HandlerFunc(s.HandleCallback))
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%v", *callbackPort), nil)
		log.Fatalf("Beerkellar is unable to serve metrics: %v", err)
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Beerkellar is unable to listen on the min grpc port %v: %v", *port, err)
	}
	gs := grpc.NewServer()
	pb.RegisterBeerKellerServer(gs, s)

	log.Printf("Serving on port :%d", *port)
	if err := gs.Serve(lis); err != nil {
		log.Fatalf("Beerkellar is unable to serve grpc for some reason: %v", err)
	}
	log.Fatalf("Beerkellar has closed the grpc port for some reason")
}
