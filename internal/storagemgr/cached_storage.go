package storagemgr

import (
	"github.com/VictoriaMetrics/fastcache"

	"github.com/axiomesh/axiom-kit/storage"
)

type CachedStorage struct {
	storage.Storage
	cache *fastcache.Cache
}

func NewCachedStorage(s storage.Storage, megabytesLimit int) (storage.Storage, error) {
	if megabytesLimit == 0 {
		megabytesLimit = 128
	}
	return &CachedStorage{
		Storage: s,
		cache:   fastcache.New(megabytesLimit * 1024 * 1024),
	}, nil
}

func (c *CachedStorage) Get(key []byte) []byte {
	value, ok := c.cache.HasGet(nil, key)
	if ok {
		if value == nil {
			return []byte{}
		}
		return value
	}

	v := c.Storage.Get(key)
	if v != nil {
		c.cache.Set(key, v)
	}
	return v
}

func (c *CachedStorage) Has(key []byte) bool {
	has := c.cache.Has(key)
	if has {
		return true
	}
	return c.Storage.Has(key)
}

func (c *CachedStorage) Put(key, value []byte) {
	if value == nil {
		value = []byte{}
	}
	c.cache.Set(key, value)
	c.Storage.Put(key, value)
}

func (c *CachedStorage) Delete(key []byte) {
	c.cache.Del(key)
	c.Storage.Delete(key)
}

func (c *CachedStorage) Close() error {
	c.cache.Reset()
	return c.Storage.Close()
}

func (c *CachedStorage) NewBatch() storage.Batch {
	return &BatchWrapper{
		Batch:      c.Storage.NewBatch(),
		cache:      c.cache,
		finalState: make(map[string][]byte),
	}
}

type BatchWrapper struct {
	storage.Batch
	cache      *fastcache.Cache
	finalState map[string][]byte
}

func (w *BatchWrapper) Put(key, value []byte) {
	if value == nil {
		value = []byte{}
	}
	w.finalState[string(key)] = value
	w.Batch.Put(key, value)
}

func (w *BatchWrapper) Delete(key []byte) {
	w.finalState[string(key)] = nil
	w.Batch.Delete(key)
}

func (w *BatchWrapper) Commit() {
	w.Batch.Commit()
	for k, v := range w.finalState {
		if v == nil {
			w.cache.Del([]byte(k))
		} else {
			w.cache.Set([]byte(k), v)
		}
	}
}
