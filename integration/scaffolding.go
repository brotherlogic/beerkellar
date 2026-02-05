package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/network"
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

// StdoutLogConsumer is a LogConsumer that prints the log to stdout
type StdoutLogConsumer struct{}

// Accept prints the log to stdout
func (lc *StdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Print(string(l.Content))
}

func runTestServer(ctx context.Context, t *testing.T) (*integrationTest, error) {

	newNetwork, err := network.New(ctx)
	require.NoError(t, err)
	//testcontainers.CleanupNetwork(t, newNetwork)
	aliases := []string{"alias1"}

	// Run the fake untappd server
	logger := log.TestLogger(t)

	uopts := []testcontainers.ContainerCustomizer{
		testcontainers.WithName("untappd"),
		testcontainers.WithExposedPorts("8085/tcp"),
		testcontainers.WithDockerfile(
			testcontainers.FromDockerfile{
				Context:    "../fake_untappd/",
				Dockerfile: "Dockerfile",
			},
		),
		testcontainers.WithLogger(logger),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("8085/tcp"),
		),
		testcontainers.WithLogConsumerConfig(&testcontainers.LogConsumerConfig{
			Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
			Consumers: []testcontainers.LogConsumer{&StdoutLogConsumer{}},
		}),
		network.WithNetwork(aliases, newNetwork),
	}
	utc, err := testcontainers.Run(ctx, "", uopts...)
	if err != nil {
		log.Printf("RUN ERROR: %v", err)
		return nil, err
	}

	ump, err := utc.MappedPort(ctx, "8085/tcp")

	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithName("beerkellar"),
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
		testcontainers.WithCmdArgs("--test_db", "--untappd_auth", fmt.Sprintf("http://localhost:%v/", ump.Int()), "--untappd_ret_auth", fmt.Sprintf("http://untappd:8085/")),
		testcontainers.WithLogConsumerConfig(&testcontainers.LogConsumerConfig{
			Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
			Consumers: []testcontainers.LogConsumer{&StdoutLogConsumer{}},
		}),

		network.WithNetwork(aliases, newNetwork),
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
		t.Fatalf("Unable to connect to server: %v -> %v", err, cmp)
	}
	defer conn.Close()
	client := pb.NewBeerKellerAdminClient(conn)
	_, err = client.SetRedirect(ctx, &pb.SetRedirectRequest{Url: fmt.Sprintf("http://beerkellar:%v", 8082)}) //cmp.Int())})
	if err != nil {
		t.Fatalf("Unable to set redirect: %v", err)
	}

	time.Sleep(time.Second * 5)
	//t.Fatalf("Running 8080->%v, 8083->%v, 8082->%v and %v", mp, amp, cmp, ump)

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
