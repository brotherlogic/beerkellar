package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type strpass struct {
	Value string
}

func get(url string, obj interface{}) error {
	path := fmt.Sprintf("%v%v", "https://api.untappd.com", url)

	resp, err := http.DefaultClient.Get(path)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("%v: %v", resp.StatusCode, string(body))
	}

	log.Printf("READ %v", len(string(body)))
	nobj := obj.(*strpass)
	nobj.Value = string(body)
	return nil
}

func (s *Server) getBeerFromUntappd(ctx context.Context, beerId int64) (*pb.Beer, error) {
	resp := &BeerInfoResponse{}
	err := get(fmt.Sprintf("/v4/beer/info/%v", beerId), resp)
	if err != nil {
		return nil, err
	}
	return &pb.Beer{
		Id:      beerId,
		Name:    resp.Bear.BeerName,
		Abv:     resp.Bear.BeerAbv,
		Brewery: resp.Bear.Brewery.BreweryName,
	}, nil
}

type BreweryResponse struct {
	BreweryName string
}

type BeerResponse struct {
	BeerName    string
	BeerAbv     float32
	Brewery     BreweryResponse
	RatingScore float32
}

type BeerInfoResponse struct {
	Bear BeerResponse
}
