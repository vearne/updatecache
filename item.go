package updatecache

import (
	"sync"
	"time"
)

type Item struct {
	key   any
	value any
	cond  *sync.Cond
	// data version
	freshFlag   bool
	cf          CalcTimeForNextUpdateFunc
	expireTimer *time.Timer
	updateTimer *time.Timer
}

func NewItem(key, value any) *Item {
	item := &Item{}
	item.key = key
	item.value = value
	item.cond = sync.NewCond(new(sync.Mutex))
	item.freshFlag = true
	return item
}
