package updatecache

import (
	"fmt"
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
	cache.UpdateLater(key, 100*time.Millisecond, func() (any, error) {
		slog.Debug("get value")
		return 2, nil
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

func TestVersion2(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 100*time.Millisecond, func() (any, error) {
		slog.Debug("--TestVersion2--")
		return 2, nil
	})
	go func() {
		time.Sleep(200 * time.Millisecond)
		cache.Set(key, 3, -1)
	}()

	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, 3, cache.Get(key), "check value")
}

func TestCancelUpdateAfter(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 200*time.Millisecond, func() (any, error) {
		slog.Debug("--TestCancelUpdateLater--")
		return 2, nil
	})
	go func() {
		time.Sleep(100 * time.Millisecond)
		cache.Set(key, 3, -1)
	}()

	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, 2, cache.Get(key), "check value")
}

func TestUpdateWait(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)
	cache.UpdateLater(key, 100*time.Millisecond, func() (any, error) {
		slog.Debug("--TestUpdateWait--")
		time.Sleep(1 * time.Second)
		return 2, nil
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
	cache.UpdateLater(key, 100*time.Millisecond, func() (any, error) {
		slog.Debug("--TestUpdateWait2--")
		time.Sleep(1 * time.Second)
		return 2, nil
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
	cache.UpdateLater(key, 100*time.Millisecond, func() (any, error) {
		slog.Debug("--TestUpdateNotWait--")
		time.Sleep(1 * time.Second)
		return 2, nil
	})

	time.Sleep(200 * time.Millisecond)
	begin := time.Now()
	value := cache.Get(key)
	cost := time.Since(begin)

	assert.Equal(t, 1, value, "check value")
	assert.Less(t, cost, time.Second,
		"don't wait GetValueFunc() finish")
}

func TestFirstLoad(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var value any
			if !cache.Contains(key) {
				value = cache.FirstLoad(key,
					1,
					func() (any, error) {
						time.Sleep(time.Second)
						slog.Debug("FirstLoad-GetValueFunc")
						return 2, nil
					},
					-1)
			} else {
				value = cache.Get(key)
			}
			assert.Equal(t, 2, value, "check value")
		}()
	}
	wg.Wait()
	value := cache.Get(key)
	assert.Equal(t, 2, value, "check value")
}

func TestFirstLoadUseDefault(t *testing.T) {
	key := "aaa"
	cache := NewCache(false)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var value any
			if !cache.Contains(key) {
				value = cache.FirstLoad(key,
					1,
					func() (any, error) {
						time.Sleep(time.Second)
						slog.Debug("FirstLoad-GetValueFunc")
						return 2, nil
					},
					-1)
			} else {
				value = cache.Get(key)
				assert.Equal(t, 1, value, "check value")
			}

		}()
	}
	wg.Wait()
	value := cache.Get(key)
	assert.Equal(t, 1, value, "check value")
}

func calcuDuration(value any) time.Duration {
	return time.Second
}
func calcuDuration2(value any) time.Duration {
	return 2 * time.Second
}
func getValue() (value any, err error) {
	return 1, nil
}

func getValue2() (value any, err error) {
	return 2, nil
}

func TestDynamicUpdateLater(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, -1)

	cache.DynamicUpdateLater(key, calcuDuration, getValue)
	cache.DynamicUpdateLater(key, calcuDuration2, getValue2)
	item, _ := cache.getItem(key)
	assert.Equal(t,
		fmt.Sprintf("%p", calcuDuration),
		fmt.Sprintf("%p", item.cf),
		"check calcuDuration function")
}
