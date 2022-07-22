package gcache

import (
	"fmt"
	"gcache/singleflight"
	"log"
	"sync"
)

// Cache  是一个缓存空间，加载的关联数据分布在上面
type Cache struct {
	//getter 当缓存找不到值的时候，就让用户决定去哪里找值
	getter    Getter
	mainCache csCache
	peers     PeerPicker
	//确保不会发出多个一样的请求
	loader *singleflight.Ones
}

// Getter 当缓存找不到值的时候，就让用户决定去哪里找值的方法的接口。
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 通过一个函数实现Getter接口。
type GetterFunc func(key string) ([]byte, error)

// Get 实现Getter接口函数
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

var (
	mu sync.RWMutex
)

// NewCache 创建一个新的Group实例
func NewCache(maxCap int, getter Getter) *Cache {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	c := &Cache{
		getter:    getter,
		mainCache: csCache{maxCap: maxCap},
		loader:    &singleflight.Ones{},
	}
	return c
}

// Get 从缓存取值
func (c *Cache) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required\n")
	}

	if v, ok := c.mainCache.get(key); ok {
		log.Printf("[gcache] hit %s\n", key)
		return v, nil
	}

	return c.load(key)
}

// RegisterPeers 注册一个PeerPicker用于选择远端对等体peer
func (c *Cache) RegisterPeers(peers PeerPicker) {
	if c.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	c.peers = peers
}

func (c *Cache) load(key string) (value ByteView, err error) {
	//每个键只获取一次(本地或远程)
	//不考虑并发调用的数量。
	viewi, err := c.loader.Do(key, func() (interface{}, error) {
		if c.peers != nil {
			//找对等peer
			if peer, ok := c.peers.PickPeer(key); ok {
				if value, err = c.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[gcache] The cache key does not yet exist")
			}
		}

		return c.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

func (c *Cache) populateCache(key string, value ByteView) {
	c.mainCache.add(key, value)
}

func (c *Cache) getLocally(key string) (ByteView, error) {
	bytes, err := c.getter.Get(key)
	if err != nil {
		return ByteView{}, err

	}
	value := ByteView{b: cloneBytes(bytes)}
	c.populateCache(key, value)
	return value, nil
}

func (c *Cache) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}
