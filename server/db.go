package server

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	pb "github.com/brotherlogic/beerkellar/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
	pspb "github.com/brotherlogic/pstore/proto"
)

type Database interface {
	GetCellar(ctx context.Context, user string) (*pb.Cellar, error)
	SaveCellar(ctx context.Context, user string, cellar *pb.Cellar) error

	GetUser(ctx context.Context, auth string) (*pb.User, error)
	SaveUser(ctx context.Context, user *pb.User) error

	GetBeer(ctx context.Context, beerid int64) (*pb.Beer, error)
	SaveBeer(ctx context.Context, beer *pb.Beer) error
}

type DB struct {
	client pstore_client.PStoreClient
}

func NewDatabase(ctx context.Context) Database {
	db := &DB{}
	client, err := pstore_client.GetClient()
	if err != nil {
		log.Fatalf("Dial error on db -> pstore: %v", err)
	}
	db.client = client

	return db
}

func NewTestDatabase(ctx context.Context) Database {
	db := &DB{}
	client := pstore_client.GetTestClient()
	db.client = client
	return db
}

func (d *DB) save(ctx context.Context, key string, message protoreflect.ProtoMessage) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	_, err = d.client.Write(ctx, &pspb.WriteRequest{
		Key:   key,
		Value: &anypb.Any{Value: data},
	})
	return err
}

func (d *DB) load(ctx context.Context, key string) ([]byte, error) {
	val, err := d.client.Read(ctx, &pspb.ReadRequest{
		Key: key,
	})
	if err != nil {
		return nil, err
	}
	return val.GetValue().GetValue(), nil
}

func (d *DB) SaveCellar(ctx context.Context, username string, cellar *pb.Cellar) error {
	return d.save(ctx, fmt.Sprintf("beerkellar/cellar/%v", username), cellar)
}

func (d *DB) GetCellar(ctx context.Context, username string) (*pb.Cellar, error) {
	data, err := d.load(ctx, fmt.Sprintf("beerkellar/cellar/%v", username))
	if err != nil {
		return nil, err
	}
	cellar := &pb.Cellar{}
	err = proto.Unmarshal(data, cellar)
	return cellar, err
}

func (d *DB) SaveUser(ctx context.Context, user *pb.User) error {
	return d.save(ctx, fmt.Sprintf("beerkellar/user/%v", user.GetAuth()), user)
}

func (d *DB) GetUser(ctx context.Context, auth string) (*pb.User, error) {
	data, err := d.load(ctx, fmt.Sprintf("beerkellar/user/%v", auth))
	if err != nil {
		return nil, err
	}
	user := &pb.User{}
	err = proto.Unmarshal(data, user)
	return user, err
}

func (d *DB) SaveBeer(ctx context.Context, beer *pb.Beer) error {
	return d.save(ctx, fmt.Sprintf("beerkellar/beer/%v", beer.GetId()), beer)
}

func (d *DB) GetBeer(ctx context.Context, beerid int64) (*pb.Beer, error) {
	data, err := d.load(ctx, fmt.Sprintf("beerkellar/beer/%v", beerid))
	if err != nil {
		return nil, err
	}
	beer := &pb.Beer{}
	err = proto.Unmarshal(data, beer)
	return beer, err
}
