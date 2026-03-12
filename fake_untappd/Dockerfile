# syntax=docker/dockerfile:1

FROM golang:1.25 AS build

WORKDIR $GOPATH/src/github.com/brotherlogic/beerkellar

COPY go.sum ./
COPY go.mod ./

RUN mkdir proto
COPY proto/*.go ./proto/

RUN go mod download

COPY fake_untappd/*.go ./

RUN CGO_ENABLED=0 go build -o /fake_untappd

##
## Deploy
##
FROM gcr.io/distroless/base-debian11

WORKDIR /

COPY --from=build /fake_untappd /fake_untappd

# HTTP
EXPOSE 8085

USER nonroot:nonroot

ENTRYPOINT ["/fake_untappd"]