# updatecache
[![golang-ci](https://github.com/vearne/updatecache/actions/workflows/golang-ci.yml/badge.svg)](https://github.com/vearne/updatecache/actions/workflows/golang-ci.yml)

## Overview
The purpose of updatecache is to update the cache conveniently.


## Feature
* prevent cache breakdown
* perform regular update of the cache
* limit the frequency of access to the upstream



## Install
```
go get github.com/vearne/updatecache
```

## Usage
use NewCache to create a local cache.
```
NewCache(waitUpdate bool, opts ...Option) *LocalCache
```
### waitUpdate
when the Get() is called
```
(c *LocalCache) Get(key any) any
```
- `true`:  If the value corresponding to a key is being updated, wait for the update function to complete and obtain the latest value
- `false`: If the value corresponding to a key is being updated, just use previous value.

### opts
WithRateLimit create a Limiter to Limit the execution frequency of get value from upstream.
```
WithRateLimit(r float64, b int) Option
```
```
c := cache.NewCache(true, cache.WithRateLimit(200, 1))
```


## Use environment variables to set log level
optional value: debug | info | warn | error
```
export SIMPLE_LOG_LEVEL=debug
```
## Unit Test
```
go test .
go test -v .
go test -run TestFirstLoad ./
```

## Example
```
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
	c := cache.NewCache(true, cache.WithRateLimit(200, 1))
	var counter uint32 = 2
	if !c.Contains(key) {
		result := c.FirstLoad(key,
			1,
			func() (any, error) {
				time.Sleep(time.Second)
				log.Println("FirstLoad-GetValueFunc")
				return 2, nil
			},
			10*time.Second)
		c.DynamicUpdateLater(key, calcuDuration, func() (any, error) {
			// get value from backend(for example: MySQL, MongoDB or other application)
			log.Println("get value from backend...")
			return atomic.AddUint32(&counter, 1), nil
		})
		log.Printf("result:%v \n", result)
	}
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		log.Println(c.Get(key))
	}
}
```