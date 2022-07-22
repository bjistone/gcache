package lru

import (
	"container/list"
)

// Cache Cache为LRU缓存,并发访问是不安全的。
type Cache struct {
	//最大容量
	maxCap int
	//已经使用了的容量
	usedMem int
	ll      *list.List
	cache   map[string]*list.Element
	//当因为缓存内存不足而清除一个k-v时执行。-->可以为nil
	OnEvicted func(key string, value Value)
}

type entry struct {
	key   string
	value Value
}

// Value 使用Len来计算需要多少字节,lru缓存的value是Value接口类型。
type Value interface {
	Len() int
}

// New 创造一个缓存。
func New(maxCap int) *Cache {
	return &Cache{
		maxCap: maxCap,
		ll:     list.New(),
		cache:  make(map[string]*list.Element),
	}
}

// Add 添加一个值到缓存。
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.usedMem += value.Len() - kv.value.Len()
		kv.value = value
	} else {
		ele := c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.usedMem += len(key) + value.Len()
	}
	for c.maxCap != 0 && c.maxCap < c.usedMem {
		c.RemoveTail()
	}
}

// Get 查找的key的值。
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// RemoveTail 根据lru的规则删除一个k-v。
func (c *Cache) RemoveTail() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.usedMem -= len(kv.key) + kv.value.Len()
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Len 返回缓存k-v的个数。
func (c *Cache) Len() int {
	return c.ll.Len()
}

func (c *Cache) Delete(key string) bool {
	var ele *list.Element
	if ele = c.cache[key]; ele == nil {
		return false
	}
	c.ll.Remove(ele)
	delete(c.cache, key)
	return true
}
