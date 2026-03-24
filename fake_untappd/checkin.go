package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func (s *Server) HandleCheckin(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling Checkin: %v", r.URL)
	// Get the beer id
	pathElems := strings.Split(r.URL.Path, "/")
	strId := pathElems[len(pathElems)-1]
	id, err := strconv.ParseInt(strId, 10, 64)
	if err != nil {
		log.Printf("Cannot parse %v", pathElems)
	}

	s.checkins = append(s.checkins, &pb.Checkin{
		BeerId:    id,
		CheckinId: time.Now().Unix(),
		Date:      time.Now().Unix(),
	})

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Read checkin: %v", id)
}

type Checkins struct {
	Count int
	Items []Checkin `json:"items"`
}

type Checkin struct {
	Checkin_id int    `json:"checkin_id"`
	Created_at string `json:"created_at"`
	Beer       Beer   `json:"beer"`
}

type Beer struct {
	Bid int `json:"bid"`
}

func (s *Server) HandleCheckins(w http.ResponseWriter, r *http.Request) {
	res := Checkins{Count: len(s.checkins)}
	for _, checkin := range s.checkins {
		res.Items = append(res.Items, Checkin{
			Checkin_id: int(checkin.GetCheckinId()),
			Created_at: time.Unix(checkin.GetDate(), 0).Format("Mon, 2 Jan 2006 15:04:05 -0700"),
			Beer:       Beer{Bid: int(checkin.GetBeerId())},
		})
	}

	jsonData, err := json.Marshal(res)
	if err != nil {
		panic(err)
	}
	_, err = w.Write(jsonData)
	log.Printf("Write Error: %v", err)
}
