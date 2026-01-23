package integration

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type integrationTest struct {
	c  testcontainers.Container
	mp int
}

func runTestServer(ctx context.Context) (*integrationTest, error) {
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
	}
	tc, err := testcontainers.Run(ctx, "", opts...)
	if err != nil {
		return nil, err
	}

	mp, err := tc.MappedPort(ctx, "8080/tcp")

	return &integrationTest{
		c:  tc,
		mp: mp.Int(),
	}, err
}

func (i *integrationTest) teardownContainer(t *testing.T) {
	testcontainers.CleanupContainer(t, i.c)
}
