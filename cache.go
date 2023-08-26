package updatecache

import (
	"context"
	slog "github.com/vearne/simplelog"
	"golang.org/x/time/rate"
	"sync"
	"sync/atomic"
	"time"
)

type GetValueFunc func() (value any, err error)

// Calculate the waiting time until the next data update
type CalcTimeForNextUpdateFunc func(value any) time.Duration

type LocalCache struct {
	m          map[any]*Item
	dataMu     sync.RWMutex
	waitUpdate bool
	// limiter Limit the execution frequency of GetValueFunc
	limiter    *rate.Limiter
	nextUpdate chan updateTask
	exitChan   chan struct{}
}

type updateTask struct {
	d            time.Duration
	key          any
	getValueFunc GetValueFunc
	c            *LocalCache
	// update for a fixed-version item
	dataVersion uint64
}

func NewCache(waitUpdate bool, opts ...Option) *LocalCache {
	c := LocalCache{}
	c.m = make(map[any]*Item)
	c.waitUpdate = waitUpdate
	c.nextUpdate = make(chan updateTask, 1)
	// Loop through each option
	for _, opt := range opts {
		opt(&c)
	}
	go func() {
		for {
			select {
			case <-c.exitChan:
				return
			case task := <-c.nextUpdate:
				slog.Debug("task:%v", task)
				item, ok := c.getItem(task.key)
				if !ok {
					return
				}
				timer := time.AfterFunc(task.d, func() {
					cacheItem, ok := c.getItem(task.key)
					if !ok {
						return
					}
					if cacheItem.dataVersion != task.dataVersion {
						slog.Debug("cacheItem.dataVersion != task.dataVersion, %v, %v",
							cacheItem.dataVersion, task.dataVersion)
						return
					}

					value, _ := task.c.oneUpdate(task.key, task.getValueFunc)
					c.nextUpdate <- updateTask{
						d:            cacheItem.cf(value),
						key:          task.key,
						getValueFunc: task.getValueFunc,
						c:            task.c,
						dataVersion:  task.dataVersion + 1,
					}
				})
				item.updateTimer = timer
			}
		}
	}()
	return &c
}

func (c *LocalCache) Stop() {
	close(c.exitChan)
}

func (c *LocalCache) FirstLoad(key any, defaultValue any, gf GetValueFunc, d time.Duration) any {
	item := NewItem(key, defaultValue)
	item.cond.L.Lock()
	item.updatingFlag = true
	item.cond.L.Unlock()

	c.dataMu.Lock()
	_, ok := c.m[key]
	if !ok {
		c.m[key] = item
		if d >= 0 {
			timer := time.AfterFunc(d, func() {
				c.dataMu.Lock()
				defer c.dataMu.Unlock()
				delete(c.m, key)
			})
			item.expireTimer = timer
		}
		go func() {
			if c.limiter != nil {
				//nolint: errcheck
				c.limiter.Wait(context.Background())
			}
			// get value from backend
			value, err := gf()
			if err == nil {
				item.cond.L.Lock()
				item.value = value
				item.updatingFlag = false
				item.cond.L.Unlock()
				item.cond.Broadcast()
			}
		}()
	}
	c.dataMu.Unlock()

	return c.Get(key)
}

/*
If d is less than 0, the item does not expire
*/
func (c *LocalCache) SetIfNotExist(key any, value any, d time.Duration) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()

	_, ok := c.m[key]
	if !ok {
		item := NewItem(key, value)
		c.m[key] = item
		if d >= 0 {
			timer := time.AfterFunc(d, func() {
				c.dataMu.Lock()
				defer c.dataMu.Unlock()
				delete(c.m, key)
			})

			item.expireTimer = timer
		}
	}
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
		item.updatingFlag = false
		item.cond.L.Unlock()
		item.cond.Broadcast()
	}

	if d >= 0 {
		timer := time.AfterFunc(d, func() {
			c.dataMu.Lock()
			defer c.dataMu.Unlock()
			delete(c.m, key)
		})
		if item.expireTimer != nil {
			item.expireTimer.Stop()
		}
		item.expireTimer = timer
	}
}

func (c *LocalCache) Contains(key any) (ok bool) {
	_, ok = c.getItem(key)
	return ok
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

	item.cond.L.Lock()
	item.cf = nil
	item.updateTimer.Stop()
	item.cond.L.Unlock()

}

func (c *LocalCache) Get(key any) any {
	item, ok := c.getItem(key)
	if !ok {
		return nil
	}

	item.cond.L.Lock()
	defer item.cond.L.Unlock()
	if c.waitUpdate {
		for item.updatingFlag {
			item.cond.Wait()
		}
	}
	return item.value
}

/*
Notice: Item and CalcTimeForNextUpdateFunc, GetValueFunc will only be bound once.
1. Calculate the waiting time until the next data update with CalcTimeForNextUpdateFunc
2. then use GetValueFunc to get value
3. update item
*/
func (c *LocalCache) DynamicUpdateLater(key any, cf CalcTimeForNextUpdateFunc, gf GetValueFunc) {
	cacheItem, ok := c.getItem(key)
	if !ok {
		return
	}

	cacheItem.cond.L.Lock()
	defer cacheItem.cond.L.Unlock()
	cacheItem.configUpdaterOnce.Do(func() {
		if cacheItem.cf == nil {
			cacheItem.cf = cf
		}
		c.nextUpdate <- updateTask{
			d:            cf(cacheItem.value),
			key:          key,
			getValueFunc: gf,
			c:            c,
			dataVersion:  cacheItem.dataVersion,
		}
	})
}

func (c *LocalCache) oneUpdate(key any, getValueFunc GetValueFunc) (any, any) {
	item, ok := c.getItem(key)
	// item may be removed
	if !ok {
		return nil, nil
	}

	item.cond.L.Lock()
	item.updatingFlag = true
	item.cond.L.Unlock()

	if c.limiter != nil {
		//nolint: errcheck
		c.limiter.Wait(context.Background())
	}
	value, err := getValueFunc()
	if err != nil {
		slog.Warn("getValueFunc(), error:%v", err)
		return value, err
	}

	item.cond.L.Lock()
	item.value = value
	atomic.AddUint64(&item.dataVersion, 1)
	item.updatingFlag = false

	slog.Debug("f():%v", item.value)
	item.cond.L.Unlock()

	item.cond.Broadcast()
	return item.value, nil
}
