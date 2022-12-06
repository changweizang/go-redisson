package go_redisson

import (
	"errors"
	"fmt"
	"gitee.com/zhouxiaozhu/go-delayqueue"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"time"
)

var WATCHDOGTIMEOUT = 30 * time.Second
var PUBLISHMESSAGE = 1
var PUBSUBCHANNEL = "publish-lock-channel"

var rLockScript = redis.NewScript(`
-- 若锁不存在，新增锁、设置锁重入次数为1、设置锁过期时间
if (redis.call('exists', 'KEYS[1]' == 0) then
	redis.call('hincrby', KEYS[1], ARGV[2], 1);
	redis.call('hexpire', KEYS[1], ARGV[1]);
	return -1;
end;
-- 若锁存在且锁标识匹配，则表明当前加锁为可重入锁，将锁重入次数+1，并在此设置锁过期时间
if (redis.call('hexists', KEYS[1], ARGV[2]) == 1) then
	redis.call('hincrby', KEYS[1], ARGV[2], 1);
	redis.call('hexpire', KEYS[1], ARGV[1]);
	return -1;
end; 
-- 若锁存在但锁标识不匹配，则表明当前锁被占用，直接返回过期时间
return redis.call('pttl', KEYS[1]);
`)

var rUnlockScript = redis.NewScript(`
-- 判断当前锁是否还是被自己持有
if (redis.call('hexists', KEYS[1], ARGV[1]) == 0) then
	return 0;
end;
-- 判断可重入次数
local count = redis.call('hincrby', KEYS[1], ARGV[1], -1);
if (count > 0) then
	redis.call('pexpire', KEYS[1], ARGV[2]);
	return 1;
else 
	redis.call('del', KEYS[1]);
	redis.call('publish', KEYS[2], ARGV[3]);
	return 2;
end;
return 0;
`)

type Rlock struct {
	Key string
	rdb *redis.Client
	uuid string
}

func InitRLock(key string, client *redis.Client) *Rlock {
	return &Rlock{
		Key: key,
		rdb: client,
		uuid: uuid.New().String(),
	}
}

func (l *Rlock) TryLock(wTime int) error {
	waitTime := int64(wTime * 1000)
	current := time.Now().UnixNano() / 1e6
	ttl, err := l.tryAcquire(-1)
	if err != nil {
		return err
	}
	// 获取锁成功
	if ttl < 0 {
		return nil
	}
	// 获取锁失败，尝试再次获取
	waitTime -= time.Now().UnixNano() / 1e6 - current
	if waitTime <= 0 {
		return errors.New("get lock failed: wait time running out")
	}
	// 等待消息
	current = time.Now().UnixNano() / 1e6
	subscribe := l.rdb.Subscribe(fmt.Sprintf("%s:%s", PUBSUBCHANNEL, l.Key))
	time.AfterFunc(time.Duration(waitTime) * time.Millisecond, func() {
		// 此时channel也会被关闭
		_ = subscribe.Close()
	})
	_, err = subscribe.ReceiveMessage()
	if err != nil {
		// 等待时间到，通道关闭
		return errors.New("retry lock failed: wait time running out")
	}
	waitTime -= time.Now().UnixNano() / 1e6 - current
	if waitTime <= 0 {
		return errors.New("retry lock failed: wait time running out")
	}
	for {
		current = time.Now().UnixNano() / 1e6
		ttl, err = l.tryAcquire(-1)
		if err != nil {
			return err
		}
		if ttl < 0 {
			return nil
		}
		waitTime -= time.Now().UnixNano() / 1e6 - current
		if waitTime <= 0 {
			return errors.New("retry lock failed: wait time running out")
		}
		current = time.Now().UnixNano() / 1e6
		if ttl < waitTime {
			time.AfterFunc(time.Duration(ttl) * time.Millisecond, func() {
				_ = subscribe.Close()
			})
			_, err = subscribe.ReceiveMessage()
			if err != nil {
				// 等待时间到，通道关闭
				return errors.New("retry lock failed: wait time running out")
			}
		} else {
			time.AfterFunc(time.Duration(waitTime) * time.Millisecond, func() {
				_ = subscribe.Close()
			})
			_, err = subscribe.ReceiveMessage()
			if err != nil {
				// 等待时间到，通道关闭
				return errors.New("retry lock failed: wait time running out")
			}
		}
		waitTime -= time.Now().UnixNano() / 1e6 - current
		if waitTime <= 0 {
			return errors.New("retry lock failed: wait time running out")
		}
	}
}

func (l *Rlock) tryAcquire(leaseTime int64) (int64, error) {
	if leaseTime != -1 {
		return l.tryAcquireInner(leaseTime)
	}
	// 默认使用看门狗的过期时间
	ttl, err := l.tryAcquireInner(WATCHDOGTIMEOUT.Milliseconds())
	if err != nil {
		return 0, err
	}
	if ttl < 0 {
		// 获取锁成功， 对锁的有效期进行续约

	}
	return ttl, nil
}

func (l *Rlock) tryAcquireInner(leaseTime int64) (int64, error) {
	gid := delayqueue.GetGoroutineID()
	lockHashKey := fmt.Sprintf("%s:%d", l.uuid, gid)
	result, err := rLockScript.Run(l.rdb, []string{l.Key}, leaseTime, lockHashKey).Result()
	if err != nil {
		return 0, err
	}
	return result.(int64), nil
}

func (l *Rlock) UnLock() error {
	gid := delayqueue.GetGoroutineID()
	publishChannel := fmt.Sprintf("%s:%s", PUBSUBCHANNEL, l.Key)
	lockHashKey := fmt.Sprintf("%s:%d", l.uuid, gid)
	_, err := rUnlockScript.Run(l.rdb, []string{l.Key, publishChannel}, lockHashKey, WATCHDOGTIMEOUT, PUBLISHMESSAGE).Result()
	if err != nil {
		return err
	}
	return nil
}















