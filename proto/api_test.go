package proto_test

import (
	"context"
	"testing"

	pb "github.com/brotherlogic/beerkellar/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	_ interface {
		RemoveBeer(context.Context, *pb.RemoveBeerRequest, ...grpc.CallOption) (*pb.RemoveBeerResponse, error)
	} = pb.BeerKellerClient(nil)

	_ interface {
		RemoveBeer(context.Context, *pb.RemoveBeerRequest) (*pb.RemoveBeerResponse, error)
	} = pb.BeerKellerServer(nil)
)

func TestRemoveBeerRequestRoundTrip(t *testing.T) {
	want := &pb.RemoveBeerRequest{
		Index:     2,
		BeerId:    12345,
		SizeFlOz:  16,
		DateAdded: 1700000000,
	}

	data, err := proto.Marshal(want)
	if err != nil {
		t.Fatalf("proto.Marshal(RemoveBeerRequest) failed: %v", err)
	}

	var got pb.RemoveBeerRequest
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("proto.Unmarshal(RemoveBeerRequest) failed: %v", err)
	}

	if !proto.Equal(want, &got) {
		t.Fatalf("round-trip mismatch: got %v, want %v", &got, want)
	}
	if got.GetIndex() != 2 {
		t.Errorf("GetIndex() = %d, want 2", got.GetIndex())
	}
	if got.GetBeerId() != 12345 {
		t.Errorf("GetBeerId() = %d, want 12345", got.GetBeerId())
	}
	if got.GetSizeFlOz() != 16 {
		t.Errorf("GetSizeFlOz() = %d, want 16", got.GetSizeFlOz())
	}
	if got.GetDateAdded() != 1700000000 {
		t.Errorf("GetDateAdded() = %d, want 1700000000", got.GetDateAdded())
	}
}

func TestRemoveBeerResponseRoundTrip(t *testing.T) {
	want := &pb.RemoveBeerResponse{}

	data, err := proto.Marshal(want)
	if err != nil {
		t.Fatalf("proto.Marshal(RemoveBeerResponse) failed: %v", err)
	}

	var got pb.RemoveBeerResponse
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("proto.Unmarshal(RemoveBeerResponse) failed: %v", err)
	}

	if !proto.Equal(want, &got) {
		t.Fatalf("round-trip mismatch: got %v, want %v", &got, want)
	}
}

func TestGetCellarResponseEntriesRoundTrip(t *testing.T) {
	want := &pb.GetCellarResponse{
		Username: "testuser",
		State:    pb.User_STATE_AUTHORIZED,
		Beers: []*pb.Beer{
			{Id: 101, Name: "Beer One", Brewery: "Brewery A", Abv: 5.5, Units: 1.95},
			{Id: 202, Name: "Beer Two", Brewery: "Brewery B", Abv: 8.0, Units: 3.78},
		},
		Entries: []*pb.CellarEntry{
			{BeerId: 101, SizeFlOz: 12, DateAdded: 1700000100},
			{BeerId: 202, SizeFlOz: 16, DateAdded: 1700000200},
		},
	}

	data, err := proto.Marshal(want)
	if err != nil {
		t.Fatalf("proto.Marshal(GetCellarResponse) failed: %v", err)
	}

	var got pb.GetCellarResponse
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("proto.Unmarshal(GetCellarResponse) failed: %v", err)
	}

	if !proto.Equal(want, &got) {
		t.Fatalf("round-trip mismatch: got %v, want %v", &got, want)
	}
	if len(got.GetEntries()) != len(got.GetBeers()) {
		t.Fatalf("len(Entries) = %d, len(Beers) = %d; want 1:1 parallel", len(got.GetEntries()), len(got.GetBeers()))
	}
	for i, wantEntry := range want.GetEntries() {
		gotEntry := got.GetEntries()[i]
		if gotEntry.GetBeerId() != wantEntry.GetBeerId() ||
			gotEntry.GetSizeFlOz() != wantEntry.GetSizeFlOz() ||
			gotEntry.GetDateAdded() != wantEntry.GetDateAdded() {
			t.Errorf("Entries[%d] = %+v, want %+v", i, gotEntry, wantEntry)
		}
	}
}

func TestBeerKellerRemoveBeerInterfaceConformance(t *testing.T) {
	const wantMethodName = "/beerkellar.BeerKeller/RemoveBeer"
	if pb.BeerKeller_RemoveBeer_FullMethodName != wantMethodName {
		t.Errorf("BeerKeller_RemoveBeer_FullMethodName = %q, want %q", pb.BeerKeller_RemoveBeer_FullMethodName, wantMethodName)
	}

	resp, err := (pb.UnimplementedBeerKellerServer{}).RemoveBeer(context.Background(), &pb.RemoveBeerRequest{})
	if resp != nil {
		t.Errorf("UnimplementedBeerKellerServer.RemoveBeer resp = %v, want nil", resp)
	}
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("UnimplementedBeerKellerServer.RemoveBeer status code = %v, want %v", status.Code(err), codes.Unimplemented)
	}
}
