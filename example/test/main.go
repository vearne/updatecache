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
	c.Set(key, 1, -1)
	var value uint32
	// optional
	c.DynamicUpdateLater(key, calcuDuration, func() any {
		// get value from backend(for example: MySQL, MongoDB or other application)
		log.Println("get value from backend...")
		atomic.AddUint32(&value, 1)
		return atomic.LoadUint32(&value)
	})
	for i := 0; i < 100; i++ {
		time.Sleep(500 * time.Millisecond)
		log.Println(c.Get(key))
	}
}
