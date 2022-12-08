package updatecache

/*
	go test ./
    go test -v ./
*/

import (
	"github.com/stretchr/testify/assert"
	slog "github.com/vearne/simplelog"
	"sync"
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, 300*time.Millisecond)
	cache.UpdateLater(key, 100*time.Millisecond, func() any {
		slog.Debug("get value")
		return 2
	})
	time.Sleep(200 * time.Millisecond)
	value := cache.Get(key)
	assert.Equal(t, 2, value, "get value from cache")
}

func TestSet2(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, 300*time.Millisecond)
	go func() {
		time.Sleep(100 * time.Millisecond)
		cache.Set(key, 2, 500*time.Millisecond)
	}()
	time.Sleep(200 * time.Millisecond)
	value := cache.Get(key)
	assert.Equal(t, 2, value, "get value from cache")
}

func TestVersion(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.Set(key, 2, -1)
	cache.Set(key, 3, -1)
	item, ok := cache.getItem(key)
	if ok {
		assert.Equal(t, uint32(2), item.version, "check version")
	}
}

func TestVersion2(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 100*time.Millisecond, func() any {
		slog.Debug("--TestVersion2--")
		return 2
	})
	go func() {
		time.Sleep(200 * time.Millisecond)
		cache.Set(key, 3, -1)
	}()

	time.Sleep(300 * time.Millisecond)
	item, ok := cache.getItem(key)
	if ok {
		assert.Equal(t, uint32(2), item.version, "check version")
	}
	assert.Equal(t, 3, cache.Get(key), "check value")
}

func TestCancelUpdateAfter(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 200*time.Millisecond, func() any {
		slog.Debug("--TestCancelUpdateLater--")
		return 2
	})
	go func() {
		time.Sleep(100 * time.Millisecond)
		cache.Set(key, 3, -1)
	}()

	time.Sleep(300 * time.Millisecond)
	item, _ := cache.getItem(key)

	assert.Equal(t, uint32(1), item.version, "check version")
	assert.Equal(t, 3, cache.Get(key), "check value")
}

func TestUpdateWait(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 100*time.Millisecond, func() any {
		slog.Debug("--TestUpdateWait--")
		time.Sleep(1 * time.Second)
		return 2
	})

	begin := time.Now()
	time.Sleep(200 * time.Millisecond)
	value := cache.Get(key)
	cost := time.Since(begin)

	assert.Equal(t, 2, value, "check value")
	assert.Greater(t, cost, time.Second,
		"wait GetValueFunc() finish")
}

func TestUpdateWait2(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 100*time.Millisecond, func() any {
		slog.Debug("--TestUpdateWait2--")
		time.Sleep(1 * time.Second)
		return 2
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			begin := time.Now()
			time.Sleep(200 * time.Millisecond)
			value := cache.Get(key)
			cost := time.Since(begin)

			assert.Equal(t, 2, value, "check value")
			assert.Greater(t, cost, time.Second,
				"wait GetValueFunc() finish")
		}()
	}
	wg.Wait()
}

func TestUpdateNotWait(t *testing.T) {
	key := "aaa"
	cache := NewCache(false)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 100*time.Millisecond, func() any {
		slog.Debug("--TestUpdateNotWait--")
		time.Sleep(1 * time.Second)
		return 2
	})

	time.Sleep(200 * time.Millisecond)
	begin := time.Now()
	value := cache.Get(key)
	cost := time.Since(begin)

	assert.Equal(t, 1, value, "check value")
	assert.Less(t, cost, time.Second,
		"don't wait GetValueFunc() finish")
}
