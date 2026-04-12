package server

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pstore_client "github.com/brotherlogic/pstore/client"
	"google.golang.org/protobuf/proto"
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
	log.Printf("GetBeer Request: %+v", req)
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
	drunks, err := s.db.GetDrunk(ctx, user.GetUserId())
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	}
	lastDrunks := make(map[int64]int64)
	if drunks != nil {
		lastDrunks = drunks.GetLastCheckins()
	}

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
		var pUnits float32
		oldest := int64(math.MaxInt64)
		leastRecent := int64(math.MaxInt64)
		for _, entry := range cellar.GetEntries() {
			if beer, ok := bcache[entry.GetBeerId()]; ok {
				units := convertToLitres(entry.GetSizeFlOz()) * beer.GetAbv()
<<<<<<< HEAD
				log.Printf("Considering %v (%v%% ABV, %voz): %v units (Limit: %v)", beer.GetName(), beer.GetAbv(), entry.GetSizeFlOz(), units, requirement.GetMaxUnits())
=======
>>>>>>> origin/main
				if requirement.GetMaxUnits() == 0 || units < requirement.GetMaxUnits() {
					ncellar = append(ncellar, entry)
					if requirement.GetStrategy() == pb.BeerRequirement_STRATEGY_OLDEST && entry.GetDateAdded() < oldest {
						oldest = entry.GetDateAdded()
						pBeer = bcache[entry.GetBeerId()]
						pUnits = units
					} else if requirement.GetStrategy() == pb.BeerRequirement_STRATEGY_LEAST_RECENTLY_DRUNK {
						lastDrunk := lastDrunks[entry.GetBeerId()]
						if lastDrunk < leastRecent {
							leastRecent = lastDrunk
							oldest = entry.GetDateAdded()
							pBeer = bcache[entry.GetBeerId()]
							pUnits = units
						} else if lastDrunk == leastRecent && entry.GetDateAdded() < oldest {
							oldest = entry.GetDateAdded()
							pBeer = bcache[entry.GetBeerId()]
							pUnits = units
						}
					}
				} else {
					log.Printf("Beer %v fails unit check: %v >= %v", entry.GetBeerId(), units, requirement.GetMaxUnits())
				}
			} else {
				log.Printf("Beer %v has no details", entry)
			}
		}

		// Pick a beer at random
		if pBeer == nil && len(ncellar) > 0 {
			log.Printf("CELLAR: %v", len(ncellar))
			pickedEntry := ncellar[rand.Intn(len(ncellar))]
			pBeer = bcache[pickedEntry.GetBeerId()]
			pUnits = convertToLitres(pickedEntry.GetSizeFlOz()) * pBeer.GetAbv()
		}

		if pBeer != nil {
			// Clone the beer so we don't modify the cache version
			nBeer := proto.Clone(pBeer).(*pb.Beer)
			nBeer.Units = pUnits

			// If we don't want to repeat beers, we can just remove it from the cache
			if req.GetNoRepeat() {
				delete(bcache, pBeer.GetId())
			}

			beers = append(beers, nBeer)
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
		if err != nil || beer.GetName() == "" {
			// Trigger a retry to cache, utilizing upgraded client
			nut := s.untappd.Upgrade(user.GetAccessToken())
			s.q.Enqueue(CacheBeer{beerId: b.BeerId, u: nut, d: s.db})

			beers = append(beers, &pb.Beer{Id: b.BeerId})
		} else {
			beers = append(beers, &pb.Beer{
				Id:      b.BeerId,
				Brewery: beer.GetBrewery(),
				Name:    beer.GetName(),
				Abv:     beer.GetAbv(),
				Units:   convertToLitres(b.GetSizeFlOz()) * beer.GetAbv(),
			})
		}
	}

	return &pb.GetCellarResponse{
		Beers:    beers,
		Username: user.GetUsername(),
		State:    user.GetState(),
	}, nil
}

func (s *Server) AddBeer(ctx context.Context, req *pb.AddBeerRequest) (*pb.AddBeerResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetSizeFlOz() <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "size_fl_oz must be specified and greater than zero")
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
	maxDate := user.GetLastFeedPull()

	drunks, err := s.db.GetDrunk(ctx, user.GetUserId())
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	}
	if drunks == nil {
		drunks = &pb.LastCheckins{LastCheckins: make(map[int64]int64)}
	}
	if drunks.LastCheckins == nil {
		drunks.LastCheckins = make(map[int64]int64)
	}

	for _, checkin := range checkins {
		if checkin.GetDate() > drunks.GetLastCheckins()[checkin.GetBeerId()] {
			drunks.LastCheckins[checkin.GetBeerId()] = checkin.GetDate()
		}

		if checkin.GetDate() > maxDate {
			maxDate = checkin.GetDate()
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
					checkin.SizeFlOz = entry.GetSizeFlOz()
				}
			}
			cellar.Entries = nbeers
		}

		err = s.db.SaveCheckin(ctx, user.GetUserId(), checkin)
		if err != nil {
			return nil, fmt.Errorf("unable to save checkins: %w", err)
		}
	}

	err = s.db.SaveDrunk(ctx, user.GetUserId(), drunks)
	if err != nil {
		return nil, err
	}

	if cellarChanged {
		err = s.db.SaveCellar(ctx, user.GetUserId(), cellar)
		if err != nil {
			return nil, err
		}
	}

	user.LastFeedPull = maxDate

	return &pb.RefreshUserResponse{}, s.db.SaveUser(ctx, user)
}

func (s *Server) GetDrunk(ctx context.Context, req *pb.GetDrunkRequest) (*pb.GetDrunkResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	checkins, err := s.db.GetCheckins(ctx, user.GetUserId())
	if err != nil {
		return nil, err
	}

	sort.Slice(checkins, func(i, j int) bool {
		return checkins[i].GetDate() > checkins[j].GetDate()
	})

	limit := req.GetCount()
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	if int32(len(checkins)) > limit {
		checkins = checkins[:limit]
	}

	var drunks []*pb.DrunkBeer
	for _, checkin := range checkins {
		beer, err := s.db.GetBeer(ctx, checkin.GetBeerId())
		if err != nil || beer.GetName() == "" {
			// Trigger a retry to cache, utilizing upgraded client
			nut := s.untappd.Upgrade(user.GetAccessToken())
			s.q.Enqueue(CacheBeer{beerId: checkin.GetBeerId(), u: nut, d: s.db})
			drunks = append(drunks, &pb.DrunkBeer{
				BeerId: checkin.GetBeerId(),
				Date:   checkin.GetDate(),
			})
		} else {
			drunks = append(drunks, &pb.DrunkBeer{
				BeerId:   checkin.GetBeerId(),
				Name:     beer.GetName(),
				Brewery:  beer.GetBrewery(),
				Abv:      beer.GetAbv(),
				SizeFlOz: checkin.GetSizeFlOz(),
				Date:     checkin.GetDate(),
				Units:    convertToLitres(checkin.GetSizeFlOz()) * beer.GetAbv(),
			})
		}
	}

	return &pb.GetDrunkResponse{Drunk: drunks}, nil
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
		// Wait for 1 minute before starting the refresh
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
			s.runRefresh(ctx)
		}

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
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
		if user.GetState() != pb.User_STATE_AUTHORIZED {
			continue
		}

		if time.Now().Unix()-user.GetLastFeedPull() > 2*3600 {
			log.Printf("Refreshing user %v", user.GetUsername())
			_, err := s.RefreshUser(ctx, &pb.RefreshUserRequest{Username: user.GetUsername()})
			if err != nil {
				log.Printf("Unable to refresh user %v: %v", user.GetUsername(), err)
			}
		}
	}
}
