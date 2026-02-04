package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type integrationTest struct {
	c  testcontainers.Container
	uc testcontainers.Container

	mp  int
	ump int
}

func runTestServer(ctx context.Context, t *testing.T) (*integrationTest, error) {

	// Run the fake untappd server
	uopts := []testcontainers.ContainerCustomizer{
		testcontainers.WithExposedPorts("8085/tcp"),
		testcontainers.WithDockerfile(
			testcontainers.FromDockerfile{
				Context:    "../fake_untappd/",
				Dockerfile: "Dockerfile",
			},
		),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("8085/tcp"),
		),
	}
	utc, err := testcontainers.Run(ctx, "", uopts...)
	if err != nil {
		log.Printf("RUN ERROR: %v", err)
		return nil, err
	}

	ump, err := utc.MappedPort(ctx, "8085/tcp")

	logger := log.TestLogger(t)
	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithExposedPorts("8080/tcp", "8082/tcp", "8083/tcp"),
		testcontainers.WithDockerfile(
			testcontainers.FromDockerfile{
				Context:    "..",
				Dockerfile: "Dockerfile",
			},
		),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("8080/tcp"),
		),
		testcontainers.WithLogger(logger),
		testcontainers.WithCmdArgs("--test_db", "--untappd_auth", fmt.Sprintf("http://localhost:%v/", ump.Int())),
	}
	tc, err := testcontainers.Run(ctx, "", opts...)
	if err != nil {
		return nil, err
	}

	mp, err := tc.MappedPort(ctx, "8080/tcp")
	if err != nil {
		return nil, err
	}
	amp, err := tc.MappedPort(ctx, "8083/tcp")
	if err != nil {
		return nil, err
	}

	cmp, err := tc.MappedPort(ctx, "8082/tcp")
	if err != nil {
		return nil, err
	}

	// Fix the redirect url
	conn, err := grpc.NewClient(fmt.Sprintf(":%v", amp.Int()), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Unable to connect to server: %v", err)
	}
	defer conn.Close()
	client := pb.NewBeerKellerAdminClient(conn)
	_, err = client.SetRedirect(ctx, &pb.SetRedirectRequest{Url: fmt.Sprintf("http://localhost:%v", cmp.Int())})
	if err != nil {
		t.Fatalf("Unable to set redirect: %v", err)
	}
	return &integrationTest{
		c:   tc,
		uc:  utc,
		mp:  mp.Int(),
		ump: ump.Int(),
	}, err
}

func (i *integrationTest) teardownContainer(t *testing.T) {
	testcontainers.CleanupContainer(t, i.c)
}
