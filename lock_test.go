package go_redisson

import (
	"github.com/go-redis/redis"
	"sync"
	"testing"
)

var wg sync.WaitGroup

func TestTryLock(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		Password: "123456",
	})
	c := InitRlock(rdb)
	lock := c.GetLock("testKey")
	err := lock.TryLock(5)
	if err != nil {
		t.Log(err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lock2 := c.GetLock("testKey")
		err = lock2.TryLock(0)
		if err != nil {
			t.Log(err)
		}
		_ = lock2.UnLock()
	}()
	wg.Wait()
	_ = lock.UnLock()

}

