package updatecache

import (
	slog "github.com/vearne/simplelog"
	"sync"
	"sync/atomic"
	"time"
)

type GetValueFunc func() any

type LocalCache struct {
	m          map[any]*Item
	dataMu     sync.RWMutex
	waitUpdate bool
}

func NewCache(waitUpdate bool) *LocalCache {
	c := LocalCache{}
	c.m = make(map[any]*Item)
	c.waitUpdate = waitUpdate
	return &c
}

func (c *LocalCache) Set(key any, value any, d time.Duration) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()

	item, ok := c.m[key]
	if !ok {
		item = NewItem(key, value)
		c.m[key] = item
	} else {
		item.cond.L.Lock()
		item.value = value
		atomic.AddUint32(&item.version, 1)
		item.cond.L.Unlock()
	}

	target := atomic.LoadUint32(&item.version)
	time.AfterFunc(d, func() {
		version := atomic.LoadUint32(&item.version)
		// item may be updated or refreshed
		if target < version {
			return
		}
		c.dataMu.Lock()
		defer c.dataMu.Unlock()
		delete(c.m, key)
	})
}

func (c *LocalCache) Get(key any) any {
	c.dataMu.RLock()
	item, ok := c.m[key]
	c.dataMu.RUnlock()

	if !ok {
		return nil
	}

	item.cond.L.Lock()
	defer item.cond.L.Unlock()
	if c.waitUpdate {
		for !item.freshFlag.IsTrue() {
			item.cond.Wait()
		}
	}
	slog.Debug("Get-item:%p", item)
	return item.value
}

// use f to get value
func (c *LocalCache) UpdateAfter(key any, d time.Duration, f GetValueFunc) {
	c.dataMu.RLock()
	item, ok := c.m[key]
	c.dataMu.RUnlock()
	if !ok {
		return
	}

	target := atomic.LoadUint32(&item.version)
	time.AfterFunc(d, func() {
		slog.Debug("[start]update item after %v", d)
		version := atomic.LoadUint32(&item.version)
		// item may be updated or refreshed
		slog.Debug("target:%v, version:%v", target, version)
		if target < version {
			return
		}
		// stale
		item.freshFlag.Set(false)

		item.cond.L.Lock()
		item.value = f()
		atomic.AddUint32(&item.version, 1)
		slog.Debug("item:%p, f():%v", item, item.value)
		item.cond.L.Unlock()

		// fresh
		item.freshFlag.Set(true)
		item.cond.Broadcast()
		slog.Debug("[end]update item after %v", d)
	})
}
