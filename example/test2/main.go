package main

import (
	cache "github.com/vearne/updatecache"
	"log"
	"sync/atomic"
	"time"
)

func calcuDuration(value any) time.Duration {
	return time.Second
}

func main() {
	key := "aaa"
	c := cache.NewCache(true)
	c.Set(key, 1, 10*time.Second)
	var counter uint32
	// optional
	c.DynamicUpdateLater(key, calcuDuration, func() (any, error) {
		// get value from backend(for example: MySQL, MongoDB or other application)
		log.Println("get value from backend...")
		return atomic.AddUint32(&counter, 1), nil
	})
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		log.Println(c.Get(key))
	}
}
