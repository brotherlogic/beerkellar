package server

import (
	"context"
	"log"
	"sync"
	"time"
)

type Queueable interface {
	run(ctx context.Context) error
}

type CacheBeer struct {
	beerId int64
	u      UntappdAPI
	d      Database
}

func (c CacheBeer) run(ctx context.Context) error {
	// Let's see if we have this in the cache already
	_, err := c.d.GetBeer(ctx, c.beerId)
	if err == nil {
		return nil
	}

	beer, err := c.u.getBeerFromUntappd(ctx, c.beerId)
	if err != nil {
		return err
	}
	log.Printf("Saving: %v", beer)
	return c.d.SaveBeer(ctx, beer)
}

type Queue struct {
	elements  []Queueable
	flushLock sync.Mutex
}

func (q *Queue) Enqueue(a Queueable) {
	q.elements = append(q.elements, a)
}

func (q *Queue) RunQueue() {
	for {
		time.Sleep(time.Second * 10)
		if len(q.elements) > 0 {
			q.flushLock.Lock()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			f := q.elements[0]
			q.elements = q.elements[1:]
			err := f.run(ctx)
			if err != nil {
				log.Printf("Unable to run queue element: %v", err)
				q.elements = append(q.elements, f)
			}
			log.Printf("Ran Queue Element %+v", f)
			cancel()
			q.flushLock.Unlock()
		}
	}
}

func (q *Queue) Flush() {
	for len(q.elements) > 0 {
		q.flushLock.Lock()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		f := q.elements[0]
		q.elements = q.elements[1:]
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
