package main

import (
	"fmt"
	cache "github.com/vearne/updatecache"
	"math/rand"
	"sync"
	"time"
)

func main() {
	key := "aaa"
	c := cache.NewCache(true)
	c.Set(key, 1, 1*time.Minute)
	// optional
	c.UpdateAfter(key, 30*time.Second, func() any {
		// get value from backend(for example: MySQL, MongoDB or other application)
		return 2
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			wg.Done()
			time.Sleep(time.Duration(rand.Intn(100)) * time.Second)
			value := c.Get(key)
			fmt.Println("value", value)
		}()
	}
	wg.Wait()
}
