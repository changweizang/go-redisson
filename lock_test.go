package go_redisson

import (
	"github.com/go-redis/redis"
	"testing"
)

func TestTryLock(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		Password: "123456",
	})
	lock := InitRLock("testKey", rdb)
	err := lock.TryLock(5)
	if err != nil {
		t.Log(err)
	}
	_ = lock.UnLock()
}
