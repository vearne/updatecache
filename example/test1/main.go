package main

import (
	lcache "github.com/vearne/updatecache"
	"time"
)

func main() {
	cache := lcache.NewCache(true)
	cache.Set("aaa", 1, time.Second)
	cache.UpdateAfter("aaa", time.Second, func() any {
		return 2
	})
}
