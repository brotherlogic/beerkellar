package server

import (
	"context"
	"log"
	"sync"
	"time"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type Queueable interface {
	run(ctx context.Context) error
}

type RefreshUser struct {
	u    UntappdAPI
	d    Database
	user *pb.User
}

func (r RefreshUser) run(ctx context.Context) error {
	checkins, err := r.u.GetCheckins(ctx)
	if err != nil {
		return err
	}

	cellar, err := r.d.GetCellar(ctx, r.user.GetUserId())

	for _, checkin := range checkins {
		err = r.d.SaveCheckin(ctx, r.user.GetUserId(), checkin)
		if err != nil {
			return err
		}

		// Remove the beer from the cellar
		var ncellar []*pb.CellarEntry
		found := false
		for _, entry := range cellar.GetEntries() {
			if found || entry.GetBeerId() != checkin.GetBeerId() {
				ncellar = append(ncellar, entry)
			}
		}
		cellar.Entries = ncellar
	}

	return r.d.SaveCellar(ctx, r.user.GetUserId(), cellar)
}

type CacheBeer struct {
	beerId int64
	u      UntappdAPI
	d      Database
	at     time.Time
}

func (c CacheBeer) run(ctx context.Context) error {
	// Let's see if we have this in the cache already
	b, err := c.d.GetBeer(ctx, c.beerId)
	if err == nil {
		log.Printf("Already have beer %v", b)
		if b.GetAbv() > 0 {
			return nil
		}
	}

	beer, err := c.u.getBeerFromUntappd(ctx, c.beerId)
	if err != nil {
		return err
	}
	log.Printf("Saving: %v", beer)
	return c.d.SaveBeer(ctx, beer)
}

type Queue struct {
	mu        sync.Mutex
	elements  []Queueable
	flushLock sync.Mutex
}

func (q *Queue) Enqueue(a Queueable) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.elements = append(q.elements, a)
}

func (q *Queue) pop() (Queueable, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.elements) == 0 {
		return nil, false
	}
	f := q.elements[0]
	q.elements[0] = nil
	q.elements = q.elements[1:]
	if len(q.elements) == 0 {
		q.elements = nil
	}
	return f, true
}

func (q *Queue) RunQueue() {
	backoff := time.Millisecond * 20
	for {
		time.Sleep(backoff)
		q.flushLock.Lock()
		f, ok := q.pop()
		if !ok {
			q.flushLock.Unlock()
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := f.run(ctx)
		if err != nil {
			backoff += time.Second
			log.Printf("Unable to run queue element: %v", err)
			q.Enqueue(f)
		} else {
			if backoff > time.Second {
				backoff -= time.Second
			}
		}
		log.Printf("Ran Queue Element %+v -> %v", f, err)
		cancel()
		q.flushLock.Unlock()
	}
}

func (q *Queue) Flush() {
	for {
		q.flushLock.Lock()
		f, ok := q.pop()
		if !ok {
			q.flushLock.Unlock()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := f.run(ctx)
		if err != nil {
			log.Printf("Unable to run queue element: %v", err)
		}
		log.Printf("Ran Queue Element %+v", f)
		cancel()
		q.flushLock.Unlock()
	}
}

func NewQueue() *Queue {
	q := &Queue{}
	go q.RunQueue()
	return q
}

func NewTestQueue() *Queue {
	q := &Queue{}
	return q
}
