package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/tasks/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func getGoogleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			tasks.TasksScope,
		},
		Endpoint: google.Endpoint,
	}
}

func (s *Server) GetGoogleLogin(ctx context.Context, req *pb.GetGoogleLoginRequest) (*pb.GetGoogleLoginResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	conf := getGoogleOAuthConfig()
	// Pass the user's auth token as state so we can identify them in the callback
	url := conf.AuthCodeURL(user.GetAuth(), oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return &pb.GetGoogleLoginResponse{Url: url}, nil
}

func (s *Server) HandleGoogleAuth(ctx context.Context, req *pb.HandleGoogleAuthRequest) (*pb.HandleGoogleAuthResponse, error) {
	// This is a placeholder since we process the code in the HandleGoogleCallback HTTP handler.
	// If the CLI provides the code directly, we can process it here.
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	conf := getGoogleOAuthConfig()
	tok, err := conf.Exchange(ctx, req.GetCode())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to exchange code: %v", err)
	}

	user.GoogleAccessToken = tok.AccessToken
	user.GoogleRefreshToken = tok.RefreshToken
	user.GoogleTasksEnabled = true

	if err := s.db.SaveUser(ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to save user: %v", err)
	}

	return &pb.HandleGoogleAuthResponse{}, nil
}

func (s *Server) ToggleGoogleTasks(ctx context.Context, req *pb.ToggleGoogleTasksRequest) (*pb.ToggleGoogleTasksResponse, error) {
	user, err := s.getUser(ctx)
	if err != nil {
		return nil, err
	}

	if user.GetGoogleAccessToken() == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "Google account is not linked")
	}

	user.GoogleTasksEnabled = req.GetEnable()
	if err := s.db.SaveUser(ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to save user: %v", err)
	}

	return &pb.ToggleGoogleTasksResponse{}, nil
}

type AddGoogleTask struct {
	user *pb.User
}

func (a AddGoogleTask) run(ctx context.Context) error {
	conf := getGoogleOAuthConfig()
	tok := &oauth2.Token{
		AccessToken:  a.user.GetGoogleAccessToken(),
		RefreshToken: a.user.GetGoogleRefreshToken(),
		Expiry:       time.Time{}, // Let the client auto-refresh
		TokenType:    "Bearer",
	}

	client := conf.Client(ctx, tok)
	srv, err := tasks.New(client)
	if err != nil {
		return fmt.Errorf("unable to create tasks client: %v", err)
	}

	// Double check we are enabled
	if !a.user.GetGoogleTasksEnabled() {
		return nil
	}

	task := &tasks.Task{
		Title: "Buy more weekday beer",
		Notes: "Your Beerkellar weekday beer count dropped below 4.",
	}

	_, err = srv.Tasks.Insert("@default", task).Do()
	if err != nil {
		return fmt.Errorf("unable to insert task: %v", err)
	}

	return nil
}