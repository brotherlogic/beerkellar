//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type mockServer struct {
	pb.UnimplementedBeerKellerServer
}

func (s *mockServer) AddBeer(ctx context.Context, req *pb.AddBeerRequest) (*pb.AddBeerResponse, error) {
	return &pb.AddBeerResponse{}, nil
}

func (s *mockServer) GetBeer(ctx context.Context, req *pb.GetBeerRequest) (*pb.GetBeerResponse, error) {
	if len(req.Requirements) > 0 && req.Requirements[0].MinUnits > 0 {
		return &pb.GetBeerResponse{Beers: []*pb.Beer{}}, nil // return nothing
	}
	return &pb.GetBeerResponse{Beers: []*pb.Beer{
		{Id: 16630, Name: "Celebration Ale", Brewery: "Sierra Nevada Brewing Co.", Abv: 6.8, Units: 2.41},
	}}, nil
}

func TestCLIWeekendPull(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	serverAddr := lis.Addr().String()

	gs := grpc.NewServer()
	pb.RegisterBeerKellerServer(gs, &mockServer{})

	go func() {
		_ = gs.Serve(lis)
	}()
	defer gs.Stop()

	// Build the beerkellar_cli binary
	tmpDir, err := os.MkdirTemp("", "cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "beerkellar_cli")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../beerkellar_cli")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build beerkellar_cli: %v, output: %s", err, string(out))
	}

	time.Sleep(1 * time.Second)

	// Configure token file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir error: %v", err)
	}
	tokenFile := filepath.Join(homeDir, ".beerkellar")
	tokenData := []byte("code: \"testuser\"\n")
	if err := os.WriteFile(tokenFile, tokenData, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	defer os.Remove(tokenFile)

	// Run `beerkellar_cli pull -weekday=false`
	cmd := exec.Command(binPath, "pull", "-weekday=false")
	cmd.Env = append(os.Environ(), fmt.Sprintf("BEERKELLAR_SERVER_ADDR=%s", serverAddr))
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v, output: %s", err, string(outBytes))
	}

	output := string(outBytes)
	
	// A weekend pull (MinUnits=3.5) should NOT pull the 6.8 ABV 12 oz beer (which is ~1.7 units)
	// So we expect it to say "No beers found matching requirements"
	if !strings.Contains(output, "No beers found matching requirements") {
		t.Errorf("Expected 'No beers found matching requirements', got: %q", output)
	}
}
