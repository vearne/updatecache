# updatecache
[![golang-ci](https://github.com/vearne/updatecache/actions/workflows/golang-ci.yml/badge.svg)](https://github.com/vearne/updatecache/actions/workflows/golang-ci.yml)

## Overview
The purpose of updatecache is to update the cache conveniently.
When executing a query to the backend to update the cache, 
other query coroutines can choose to wait for the query to complete or
directly use the value in the current cache.

## Install
```
go get github.com/vearne/updatecache
```

## Use environment variables to set log level
optional value: debug | info | warn | error
```
export SIMPLE_LOG_LEVEL=debug
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
	c := cache.NewCache(true)
	c.Set(key, 1, 30*time.Second)
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
```