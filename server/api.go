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

	user, err := s.db.GetUser(ctx, key)
	if err != nil {
		return nil, err
	}

	if user.GetState() != pb.User_STATE_AUTHORIZED {
		return nil, status.Errorf(codes.PermissionDenied, "User is not authorized (current state: %v)", user.GetState())
	}

	return user, nil
}

func (s *Server) GetBeer(ctx context.Context, req *pb.GetBeerRequest) (*pb.GetBeerResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	cellar, err := s.db.GetCellar(ctx, user.GetUserId())
	if err != nil {
		if status.Code(err) == codes.NotFound && user.GetUserId() > 0 {
			cellar = &pb.Cellar{}
			err = s.db.SaveCellar(ctx, user.GetUserId(), cellar)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
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
		if status.Code(err) == codes.NotFound && user.GetUserId() > 0 {
			cellar = &pb.Cellar{}
			err = s.db.SaveCellar(ctx, user.GetUserId(), cellar)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
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
		if status.Code(err) == codes.NotFound && user.GetUserId() > 0 {
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
		Auth:  tmpToken,
		State: pb.User_STATE_LOGGING_IN,
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

	if user.GetState() == pb.User_STATE_LOGGED_IN {
		nut := s.untappd.Upgrade(user.GetAccessToken())
		username, userId, err := nut.GetUserInfo(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Unavailable, "Unable to fetch user info: %v (retryable)", err)
		}

		user.Username = username
		user.UserId = userId
		user.State = pb.User_STATE_AUTHORIZED
		err = s.db.SaveUser(ctx, user)
		if err != nil {
			return nil, err
		}
	}

	if user.GetState() == pb.User_STATE_AUTHORIZED {
		return &pb.GetAuthTokenResponse{
			Code: req.GetCode(),
		}, nil
	}
	return &pb.GetAuthTokenResponse{}, status.Errorf(codes.NotFound, "User is not fully authenticated (current state: %v)", user.GetState())
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
	// Get the user
	user, err := s.db.GetUserByName(ctx, req.GetUsername())
	if err != nil {
		return nil, fmt.Errorf("unable to locate user %v -> %w", req.GetUsername(), err)
	}

	if user.GetState() != pb.User_STATE_AUTHORIZED {
		return nil, status.Errorf(codes.PermissionDenied, "User %v is not logged in", req.GetUsername())
	}

	nut := s.untappd.Upgrade(user.GetAccessToken())

	checkins, err := nut.GetCheckins(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get checkins: %w", err)
	}

	cellar, err := s.db.GetCellar(ctx, user.GetUserId())
	cellarChanged := false
	if err != nil {
		if status.Code(err) == codes.NotFound && user.GetUserId() > 0 {
			cellar = &pb.Cellar{}
		} else {
			return nil, fmt.Errorf("unable to get cellar: %w", err)
		}
	}

	log.Printf("Found %v checkins", len(checkins))
	for _, checkin := range checkins {
		err = s.db.SaveCheckin(ctx, user.GetUserId(), checkin)
		if err != nil {
			return nil, fmt.Errorf("unable to save checkins: %w", err)
		}

		// Remove the beer from the cellar if it's recent
		if checkin.GetDate() > user.GetLastFeedPull() {
			var nbeers []*pb.CellarEntry
			found := false
			for _, entry := range cellar.GetEntries() {
				if found || entry.GetBeerId() != checkin.GetBeerId() {
					nbeers = append(nbeers, entry)
				} else {
					found = true
					cellarChanged = true
				}
			}
			cellar.Entries = nbeers
		}
	}

	if cellarChanged {
		err = s.db.SaveCellar(ctx, user.GetUserId(), cellar)
		if err != nil {
			return nil, err
		}
	}

	user.LastFeedPull = time.Now().Unix()

	return &pb.RefreshUserResponse{}, s.db.SaveUser(ctx, user)
}

func (s *Server) GetDrunk(ctx context.Context, req *pb.GetDrunkRequest) (*pb.GetDrunkResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	drunks, err := s.db.GetDrunk(ctx, user.GetUserId())
	if err != nil {
		if status.Code(err) == codes.NotFound && user.GetUserId() > 0 {
			drunks = &pb.LastCheckins{LastCheckins: make(map[int64]int64)}
			err = s.db.SaveDrunk(ctx, user.GetUserId(), drunks)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &pb.GetDrunkResponse{Drunk: drunks.GetLastCheckins()}, nil
}
func (s *Server) DrinkBeer(ctx context.Context, req *pb.DrinkBeerRequest) (*pb.DrinkBeerResponse, error) {
	err := s.untappd.Checkin(ctx, req.GetBeerId())
	if err != nil {
		return nil, err
	}

	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.RefreshUser(ctx, &pb.RefreshUserRequest{Username: user.GetUsername()})
	return &pb.DrinkBeerResponse{}, err
}

func (s *Server) StartBackgroundTasks(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runRefresh(ctx)
			}
		}
	}()
}

func (s *Server) runRefresh(ctx context.Context) {
	users, err := s.db.GetUsers(ctx)
	if err != nil {
		log.Printf("Unable to get users for refresh: %v", err)
		return
	}

	for _, user := range users {
		if time.Now().Unix()-user.GetLastFeedPull() > 2*3600 {
			log.Printf("Refreshing user %v", user.GetUsername())
			_, err := s.RefreshUser(ctx, &pb.RefreshUserRequest{Username: user.GetUsername()})
			if err != nil {
				log.Printf("Unable to refresh user %v: %v", user.GetUsername(), err)
			}
		}
	}
}
