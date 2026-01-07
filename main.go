package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beerkellar/proto"
	"github.com/brotherlogic/beerkellar/server"
)

var (
	port         = flag.Int("port", 8080, "Server port for grpc traffic")
	metricsPort  = flag.Int("metrics_port", 8081, "Metrics port")
	clientId     = flag.String("client_id", "", "Client Id From Untappd")
	clientSecret = flag.String("client_secret", "", "Client Secret From Untappd")
	redirectUrl  = flag.String("redirect_url", "", "Redirect Url From Untappd")
)

func main() {
	flag.Parse()

	s := server.NewServer(*clientId, *clientSecret, *redirectUrl)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%v", *metricsPort), nil)
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
