package server

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pstore_client "github.com/brotherlogic/pstore/client"
)

const (
	CONFIG = "github.com/brotherlogic/beerkellar/config"
	CACHE  = "github.com/brotherlogic/beerkellar/cache"
)

type Server struct {
	client pstore_client.PStoreClient
	db     Database

	clientId     string
	clientSecret string
	redirectUrl  string

	untappd UntappdAPI

	q *Queue
}

func NewServer(clientId, clientSecret, redirectUrl string, db Database, ut *Untappd) *Server {
	return &Server{
		clientId:     clientId,
		clientSecret: clientSecret,
		redirectUrl:  redirectUrl,

		db:      db,
		untappd: ut,

		q: NewQueue(),
	}
}

func (s *Server) loadConfig(ctx context.Context) (*pb.Cellar, error) {
	return &pb.Cellar{}, nil
}

func (s *Server) getBeerFromCache(ctx context.Context, beerId int64) (*pb.Beer, error) {
	beer, err := s.db.GetBeer(ctx, beerId)
	if err == nil {
		return beer, nil
	}

	// Cache miss - call out to Untappd
	return s.untappd.getBeerFromUntappd(ctx, beerId)
}

func GetContextKey(ctx context.Context) (string, error) {
	md, found := metadata.FromIncomingContext(ctx)
	if found {
		if _, ok := md["auth-token"]; ok {
			idt := md["auth-token"][0]

			if idt != "" {
				return idt, nil
			}
		}
	}

	md, found = metadata.FromOutgoingContext(ctx)
	if found {
		if _, ok := md["auth-token"]; ok {
			idt := md["auth-token"][0]

			if idt != "" {
				return idt, nil
			}
		}
	}

	return "", status.Errorf(codes.NotFound, "Could not extract token from incoming or outgoing")
}

func (s *Server) getUser(ctx context.Context) (*pb.User, error) {
	key, err := GetContextKey(ctx)
	if err != nil {
		return nil, err
	}

	return s.db.GetUser(ctx, key)
}

func (s *Server) GetBeer(ctx context.Context, req *pb.GetBeerRequest) (*pb.GetBeerResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	cellar, err := s.db.GetCellar(ctx, user.GetUserId())
	if err != nil {
		return nil, err
	}

	log.Printf("Found: %v", cellar)

	bcache := make(map[int64]*pb.Beer)
	for _, entry := range cellar.GetEntries() {
		beer, err := s.db.GetBeer(ctx, entry.GetBeerId())
		if err == nil {
			bcache[beer.GetId()] = beer
		}
	}

	var beers []*pb.Beer
	for _, requirement := range req.GetRequirements() {

		// Filter out beers
		var ncellar []*pb.CellarEntry
		var pBeer *pb.Beer
		oldest := int64(math.MaxInt64)
		for _, entry := range cellar.GetEntries() {
			if beer, ok := bcache[entry.GetBeerId()]; ok {
				if requirement.GetMaxUnits() == 0 || convertToLitres(entry.GetSizeFlOz())*beer.GetAbv() < float32(requirement.GetMaxUnits()) {
					ncellar = append(ncellar, entry)
					if requirement.GetStrategy() == pb.BeerRequirement_STRATEGY_OLDEST && entry.GetDateAdded() < oldest {
						oldest = entry.GetDateAdded()
						pBeer = bcache[entry.GetBeerId()]
					}
				}
			} else {
				log.Printf("Beer %v has no details", entry)
			}
		}

		// Pick a beer at random
		if pBeer == nil && len(ncellar) > 0 {
			log.Printf("CELLAR: %v", len(ncellar))
			pBeer = bcache[ncellar[rand.Intn(len(ncellar))].GetBeerId()]
		}

		// If we don't want to repeat beers, we can just remove it from the cache
		if req.GetNoRepeat() {
			delete(bcache, pBeer.GetId())
		}
		if pBeer != nil {
			beers = append(beers, pBeer)
		}
	}
	return &pb.GetBeerResponse{Beers: beers}, nil
}

func (s *Server) GetCellar(ctx context.Context, _ *pb.GetCellarRequest) (*pb.GetCellarResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	cellar, err := s.db.GetCellar(ctx, user.GetUserId())
	if err != nil {
		return nil, err
	}

	var beers []*pb.Beer
	for _, b := range cellar.GetEntries() {
		beer, err := s.db.GetBeer(ctx, b.BeerId)
		if err != nil {
			// Trigger a retry to cache
			s.q.Enqueue(&CacheBeer{beerId: b.BeerId, u: s.untappd, d: s.db})

			beers = append(beers, &pb.Beer{Id: b.BeerId})
		} else {
			beers = append(beers, &pb.Beer{Id: b.BeerId, Brewery: beer.GetBrewery(), Name: beer.GetName(), Abv: beer.GetAbv()})
		}
	}

	return &pb.GetCellarResponse{Beers: beers}, nil
}

func (s *Server) AddBeer(ctx context.Context, req *pb.AddBeerRequest) (*pb.AddBeerResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}
	cellar, err := s.db.GetCellar(ctx, user.GetUserId())
	if err != nil {
		// OutOfRange is cannot be found in DB
		if status.Code(err) == codes.NotFound {
			cellar = &pb.Cellar{}
		} else {
			return nil, err
		}
	}

	for range req.GetQuantity() {
		cellar.Entries = append(cellar.Entries,
			&pb.CellarEntry{
				BeerId:    req.GetBeerId(),
				SizeFlOz:  req.GetSizeFlOz(),
				DateAdded: time.Now().Unix(),
			})
	}

	s.q.Enqueue(CacheBeer{
		beerId: req.GetBeerId(),
		u:      s.untappd.Upgrade(user.GetAccessToken()),
		d:      s.db})

	return &pb.AddBeerResponse{}, s.db.SaveCellar(ctx, user.GetUserId(), cellar)
}

func (s *Server) GetLogin(ctx context.Context, req *pb.GetLoginRequest) (*pb.GetLoginResponse, error) {
	tmpToken := fmt.Sprintf("%v-%v", time.Now().UnixNano(), rand.Int63())
	user := &pb.User{
		Auth: tmpToken,
	}
	err := s.db.SaveUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return &pb.GetLoginResponse{
		Url:  fmt.Sprintf("%voauth/authenticate?client_id=%v&response_type=code&redirect_url=%v&state=%v", s.untappd.getBaseAuthURL(), s.clientId, s.redirectUrl, user.GetAuth()),
		Code: user.GetAuth(),
	}, nil
}

func (s *Server) GetAuthToken(ctx context.Context, req *pb.GetAuthTokenRequest) (*pb.GetAuthTokenResponse, error) {
	user, err := s.db.GetUser(ctx, req.GetCode())
	if err != nil {
		return nil, err
	}

	if len(user.GetAccessToken()) > 0 {
		return &pb.GetAuthTokenResponse{
			Code: req.GetCode(),
		}, nil
	}
	return &pb.GetAuthTokenResponse{}, status.Errorf(codes.NotFound, "User is not fully authenticated")
}

func convertToLitres(flOz int32) float32 {
	return float32(flOz) * 0.029574
}

func (s *Server) Healthy(_ context.Context, _ *pb.HealthyRequest) (*pb.HealthyResponse, error) {
	return &pb.HealthyResponse{}, nil
}

func (s *Server) SetRedirect(_ context.Context, req *pb.SetRedirectRequest) (*pb.SetRedirectResponse, error) {
	s.redirectUrl = req.GetUrl()

	log.Printf("Adjusted redirect: %v", req)

	return &pb.SetRedirectResponse{}, nil
}

func (s *Server) RefreshUser(ctx context.Context, req *pb.RefreshUserRequest) (*pb.RefreshUserResponse, error) {
	return &pb.RefreshUserResponse{}, status.Errorf(codes.Unimplemented, "Nto implemented yet")
}

func (s *Server) GetDrunk(ctx context.Context, req *pb.GetDrunkRequest) (*pb.GetDrunkResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	drunks, err := s.db.GetDrunk(ctx, user.GetUserId())
	if err != nil {
		return nil, err
	}

	return &pb.GetDrunkResponse{Drunk: drunks.GetLastCheckins()}, nil
}
