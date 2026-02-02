package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type integrationTest struct {
	c  testcontainers.Container
	uc testcontainers.Container

	mp  int
	ump int
}

func runTestServer(ctx context.Context) (*integrationTest, error) {

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
		return nil, err
	}

	ump, err := utc.MappedPort(ctx, "8085/tcp")

	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithExposedPorts("8080/tcp"),
		testcontainers.WithDockerfile(
			testcontainers.FromDockerfile{
				Context:    "..",
				Dockerfile: "Dockerfile",
			},
		),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("8080/tcp"),
		),
		testcontainers.WithCmdArgs("--test_db"),
		testcontainers.WithCmdArgs(fmt.Sprintf("--baseUntappdAuth http://localhost:%v/", ump)),
	}
	tc, err := testcontainers.Run(ctx, "", opts...)
	if err != nil {
		return nil, err
	}

	mp, err := tc.MappedPort(ctx, "8080/tcp")
	if err != nil {
		return nil, err
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
