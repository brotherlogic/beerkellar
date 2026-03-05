package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
)

func (s *Server) HandleCheckin(w http.ResponseWriter, r *http.Request) {
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
	fmt.Fprint(w, "Great")
}
