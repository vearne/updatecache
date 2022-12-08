package updatecache

import (
	slog "github.com/vearne/simplelog"
	"sync"
	"sync/atomic"
	"time"
)

type GetValueFunc func() any

//Calculate the waiting time until the next data update
type CalcTimeForNextUpdateFunc func(value any) time.Duration

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

/*
	If d is less than 0, the item does not expire
*/
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
		// fresh
		item.freshFlag = true
		item.cond.L.Unlock()
		item.cond.Broadcast()
	}

	if d >= 0 {
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
}

func (c *LocalCache) getItem(key any) (item *Item, ok bool) {
	c.dataMu.RLock()
	item, ok = c.m[key]
	c.dataMu.RUnlock()
	return item, ok
}

func (c *LocalCache) Size() int {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	return len(c.m)
}

func (c *LocalCache) Remove(key any) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()
	delete(c.m, key)
}

func (c *LocalCache) StopLaterUpdate(key any) {
	item, ok := c.getItem(key)
	if !ok {
		return
	}

	item.cf = nil
	atomic.AddUint32(&item.version, 1)
}

func (c *LocalCache) Get(key any) any {
	item, ok := c.getItem(key)
	if !ok {
		return nil
	}

	item.cond.L.Lock()
	defer item.cond.L.Unlock()
	if c.waitUpdate {
		for !item.freshFlag {
			item.cond.Wait()
		}
	}
	slog.Debug("Get-item:%p", item)
	return item.value
}

/*
	1. Calculate the waiting time until the next data update with CalcTimeForNextUpdateFunc
	2. then use GetValueFunc to get value
	3. update item
*/
func (c *LocalCache) DynamicUpdateLater(key any, cf CalcTimeForNextUpdateFunc, gf GetValueFunc) {
	item, ok := c.getItem(key)
	if !ok {
		return
	}
	item.cf = cf
	c.UpdateLater(key, cf(item.value), gf)
}

//  1. wait d
//  2. then use GetValueFunc to get value
//  3. update item
func (c *LocalCache) UpdateLater(key any, d time.Duration, getValueFunc GetValueFunc) {
	if d < 0 {
		return
	}

	item, ok := c.getItem(key)
	if !ok {
		return
	}

	target := atomic.LoadUint32(&item.version)
	time.AfterFunc(d, func() {
		slog.Debug("[start]update item after %v", d)
		item, ok := c.getItem(key)
		// item may be removed
		if !ok {
			return
		}

		version := atomic.LoadUint32(&item.version)
		// item may be updated or refreshed
		slog.Debug("target:%v, version:%v", target, version)
		if target < version {
			return
		}
		// stale
		item.cond.L.Lock()
		item.freshFlag = false
		item.cond.L.Unlock()

		value := getValueFunc()

		item.cond.L.Lock()
		item.value = value
		atomic.AddUint32(&item.version, 1)
		// fresh
		item.freshFlag = true
		slog.Debug("item:%p, f():%v", item, item.value)
		item.cond.L.Unlock()

		item.cond.Broadcast()

		// plan for next update
		if item.cf != nil {
			c.UpdateLater(key, item.cf(value), getValueFunc)
		}

		slog.Debug("[end]update item after %v", d)
	})
}
