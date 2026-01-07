# syntax=docker/dockerfile:1

FROM golang:1.24 AS build

WORKDIR $GOPATH/src/github.com/brotherlogic/beerkellar

COPY go.mod ./
COPY go.sum ./

RUN mkdir proto
COPY proto/*.go ./proto/

RUN mkdir server
COPY server/*.go ./server/

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 go build -o /beerkellar

##
## Deploy
##
FROM gcr.io/distroless/base-debian11

WORKDIR /

COPY --from=build /beerkellar /beerkellar

# HTTP ; GRPC ; Metrics
EXPOSE 80   
EXPOSE 8080
EXPOSE 8081

USER nonroot:nonroot

ENTRYPOINT ["/beerkellar"]