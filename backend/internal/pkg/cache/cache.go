package cache

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var ErrNegative = errors.New("cache: negative entry")

type Loader[K comparable, V any] func(context.Context, K) (V, error)

type Cache[K comparable, V any] interface {
	Get(K) (V, bool)
	Set(K, V, time.Duration)
	Delete(K)
	Clear()
	Len() int
}

type Options struct {
	MaxEntries int
	DefaultTTL time.Duration
	Now        func() time.Time
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
	negative  bool
}

type memoryCache[K comparable, V any] struct {
	mu         sync.Mutex
	items      map[K]*list.Element
	order      *list.List
	maxEntries int
	defaultTTL time.Duration
	now        func() time.Time
	group      singleflight.Group
}

func New[K comparable, V any](opts Options) *memoryCache[K, V] {
	if opts.MaxEntries < 1 {
		opts.MaxEntries = 1
	}
	if opts.DefaultTTL <= 0 {
		opts.DefaultTTL = time.Minute
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &memoryCache[K, V]{
		items:      map[K]*list.Element{},
		order:      list.New(),
		maxEntries: opts.MaxEntries,
		defaultTTL: opts.DefaultTTL,
		now:        opts.Now,
	}
}

func (c *memoryCache[K, V]) Get(key K) (V, bool) {
	value, ok, negative := c.getEntry(key)
	if negative {
		var zero V
		return zero, false
	}
	return value, ok
}

func (c *memoryCache[K, V]) GetEntry(key K) (value V, ok bool, negative bool) {
	return c.getEntry(key)
}

func (c *memoryCache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.set(key, value, ttl, false)
}

func (c *memoryCache[K, V]) SetNegative(key K, ttl time.Duration) {
	var zero V
	c.set(key, zero, ttl, true)
}

func (c *memoryCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteLocked(key)
}

func (c *memoryCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = map[K]*list.Element{}
	c.order.Init()
}

func (c *memoryCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpiredLocked(c.now())
	return len(c.items)
}

func (c *memoryCache[K, V]) Load(ctx context.Context, key K, ttl time.Duration, loader Loader[K, V]) (V, error) {
	if value, ok, negative := c.getEntry(key); ok || negative {
		if negative {
			var zero V
			return zero, ErrNegative
		}
		return value, nil
	}
	result, err, _ := c.group.Do(fmt.Sprint(key), func() (any, error) {
		if value, ok, negative := c.getEntry(key); ok || negative {
			if negative {
				var zero V
				return zero, ErrNegative
			}
			return value, nil
		}
		value, loadErr := loader(ctx, key)
		if loadErr != nil {
			var zero V
			return zero, loadErr
		}
		c.Set(key, value, ttl)
		return value, nil
	})
	if err != nil {
		var zero V
		return zero, err
	}
	value, ok := result.(V)
	if !ok {
		var zero V
		return zero, fmt.Errorf("cache: loader returned unexpected value type")
	}
	return value, nil
}

func ScopeKey(tenantID int64, shopID string, parts ...string) (string, error) {
	if tenantID <= 0 {
		return "", fmt.Errorf("cache tenant scope is required")
	}
	clean := []string{fmt.Sprintf("tenant:%d", tenantID)}
	if strings.TrimSpace(shopID) != "" {
		clean = append(clean, "shop:"+strings.TrimSpace(shopID))
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clean = append(clean, strings.ReplaceAll(part, "|", "_"))
	}
	return strings.Join(clean, "|"), nil
}

func (c *memoryCache[K, V]) getEntry(key K) (value V, ok bool, negative bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	c.evictExpiredLocked(now)
	elem := c.items[key]
	if elem == nil {
		return value, false, false
	}
	c.order.MoveToFront(elem)
	item := elem.Value.(*entry[K, V])
	return item.value, !item.negative, item.negative
}

func (c *memoryCache[K, V]) set(key K, value V, ttl time.Duration, negative bool) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpiredLocked(c.now())
	if elem := c.items[key]; elem != nil {
		item := elem.Value.(*entry[K, V])
		item.value = value
		item.expiresAt = c.now().Add(ttl)
		item.negative = negative
		c.order.MoveToFront(elem)
		return
	}
	elem := c.order.PushFront(&entry[K, V]{
		key:       key,
		value:     value,
		expiresAt: c.now().Add(ttl),
		negative:  negative,
	})
	c.items[key] = elem
	for len(c.items) > c.maxEntries {
		back := c.order.Back()
		if back == nil {
			break
		}
		item := back.Value.(*entry[K, V])
		delete(c.items, item.key)
		c.order.Remove(back)
	}
}

func (c *memoryCache[K, V]) evictExpiredLocked(now time.Time) {
	for elem := c.order.Back(); elem != nil; {
		prev := elem.Prev()
		item := elem.Value.(*entry[K, V])
		if !item.expiresAt.After(now) {
			delete(c.items, item.key)
			c.order.Remove(elem)
		}
		elem = prev
	}
}

func (c *memoryCache[K, V]) deleteLocked(key K) {
	elem := c.items[key]
	if elem == nil {
		return
	}
	delete(c.items, key)
	c.order.Remove(elem)
}
