package reverseproxy

import (
	"fmt"
	cmap "github.com/orcaman/concurrent-map/v2"
	"log"
	"math"
	"sync"
	"time"
)

var UpstreamWeightMap = cmap.New[*UpstreamWeight]()

type UpstreamWeight struct {
	Id      string
	Expire  int64
	Weight  int
	rwMutex *sync.RWMutex
}

func NewUpstreamWeight(id string) *UpstreamWeight {
	return &UpstreamWeight{Id: id, Expire: nextResetWeightTimestamp(), Weight: 0, rwMutex: new(sync.RWMutex)}
}

// 当前权重记录是否已过期
func (u *UpstreamWeight) isExpired() bool {
	u.rwMutex.RLock()
	defer u.rwMutex.RUnlock()
	return u.Expire <= time.Now().UnixMilli()
}

func (u *UpstreamWeight) Reset() {
	u.rwMutex.Lock()
	u.Weight = 0
	u.Expire = nextResetWeightTimestamp()
	u.rwMutex.Unlock()
}

func (u *UpstreamWeight) ReadWeight() int {
	u.rwMutex.RLock()
	defer u.rwMutex.RUnlock()
	return u.Weight
}

func (u *UpstreamWeight) updateWeight(weight int) {
	u.rwMutex.RLock()
	if weight <= u.Weight {
		u.rwMutex.RUnlock()
		return
	}
	u.rwMutex.RUnlock()
	u.rwMutex.Lock()
	u.Weight = weight
	u.rwMutex.Unlock()
}

func (u *UpstreamWeight) Destruct() error {
	return nil
}

func (u *UpstreamWeight) String() string {
	u.rwMutex.RLock()
	defer u.rwMutex.RUnlock()
	return fmt.Sprintf("Id=%s, Weight=%d, Expire=%s", u.Id, u.Weight, time.UnixMilli(u.Expire))
}

func nextResetWeightTimestamp() int64 {
	now := time.Now()
	// 截取分钟之后的时间 + 1分钟偏移 输出为为毫秒
	return now.Truncate(time.Minute).Add(time.Minute).UnixMilli()
}

func UpdateBinanceUsedWeight(id string, weight int) {
	value, exist := UpstreamWeightMap.Get(id)
	if !exist {
		return
	}
	value.updateWeight(weight)
}

func SelectMinimumWeight(pool UpstreamPool) *Upstream {
	minIndex := -1
	minWeight := math.MaxInt
	for index, upstream := range pool {
		id := upstream.String()
		value, exist := UpstreamWeightMap.Get(id)
		if !exist {
			value = UpstreamWeightMap.Upsert(id, nil, func(exist bool, valueInMap *UpstreamWeight, newValue *UpstreamWeight) *UpstreamWeight {
				if !exist {
					return NewUpstreamWeight(id)
				}
				return valueInMap
			})
		}
		if value.isExpired() {
			// 重置权重和到期时间
			value.Reset()
		}
		weight := value.ReadWeight()
		if weight < minWeight {
			minWeight = weight
			minIndex = index
		}
	}
	if minIndex < 0 {
		return nil
	}
	return pool[minIndex]
}

func PrintOverview() {
	for t := range UpstreamWeightMap.IterBuffered() {
		log.Println(t.Val.String())
	}
}
