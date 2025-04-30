package test

import (
	"sync"
	"time"
)

type Repository struct {
	Header Header
	Body   []*Body
}

type Body struct {
	Key    []byte
	Last   int64
	Excess int64
}

type Header struct {
	Key          string
	Now          int64
	NowMonotonic int64
}

type Repositories struct {
	Mutex  sync.Mutex
	Values map[string]*Repository
}

func NewRepository() *Repositories {
	now := time.Now().UTC().UnixMilli()
	values := map[string]*Repository{
		"one": {
			Header: Header{
				Key:          "$remote_addr",
				Now:          now,
				NowMonotonic: now,
			},
			Body: []*Body{},
		},
		"two": {
			Header: Header{
				Key:          "$remote_addr",
				Now:          now,
				NowMonotonic: now,
			},
			Body: []*Body{},
		},
	}
	return &Repositories{
		Values: values,
	}
}

func (r *Repositories) GetRateLimit() map[string]*Repository {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	return r.Values
}

func (r *Repositories) SetRateLimit(zone string, values []*Body) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	_, ok := r.Values[zone]
	if ok {
		zone := r.Values[zone]
		zone.Body = values
		return
	}
}
