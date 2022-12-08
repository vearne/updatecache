# updatecache

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
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, 1*time.Minute)
	// optional
	cache.UpdateAfter(key, 30*time.Second, func() any {
		// get value from backend(for example: MySQL, MongoDB or other application)
		return 2
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			wg.Done()
			time.Sleep(time.Duration(rand.Intn(100)) * time.Second)
			value := cache.Get(key)
			fmt.Println("value", value)
		}()
	}
	wg.Wait()
```