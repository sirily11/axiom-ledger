package storagemgr

import (
	"fmt"
	"sync"

	"github.com/axiomesh/axiom-kit/storage"
	"github.com/axiomesh/axiom-kit/storage/leveldb"
	"github.com/axiomesh/axiom-kit/storage/pebble"
	"github.com/axiomesh/axiom-ledger/pkg/repo"
)

const (
	BlockChain = "blockchain"
	Ledger     = "ledger"
	Blockfile  = "blockfile"
	Consensus  = "consensus"
	Epoch      = "epoch"
)

var globalStorageMgr = &storageMgr{
	storageBuilderMap: make(map[string]func(p string) (storage.Storage, error)),
	storages:          make(map[string]storage.Storage),
	lock:              new(sync.Mutex),
}

type storageMgr struct {
	storageBuilderMap map[string]func(p string) (storage.Storage, error)
	storages          map[string]storage.Storage
	defaultKVType     string
	lock              *sync.Mutex
}

func (m *storageMgr) Open(typ string, p string) (storage.Storage, error) {
	builder, ok := m.storageBuilderMap[typ]
	if !ok {
		return nil, fmt.Errorf("unknow kv type %s, expect leveldb or pebble", typ)
	}
	return builder(p)
}

func Initialize(defaultKVType string) error {
	globalStorageMgr.storageBuilderMap[repo.KVStorageTypeLeveldb] = func(p string) (storage.Storage, error) {
		return leveldb.New(p, nil)
	}
	globalStorageMgr.storageBuilderMap[repo.KVStorageTypePebble] = func(p string) (storage.Storage, error) {
		return pebble.New(p, nil)
	}
	_, ok := globalStorageMgr.storageBuilderMap[defaultKVType]
	if !ok {
		return fmt.Errorf("unknow kv type %s, expect leveldb or pebble", defaultKVType)
	}
	globalStorageMgr.defaultKVType = defaultKVType
	return nil
}

func Open(p string) (storage.Storage, error) {
	return OpenSpecifyType(globalStorageMgr.defaultKVType, p)
}

func OpenSpecifyType(typ string, p string) (storage.Storage, error) {
	globalStorageMgr.lock.Lock()
	defer globalStorageMgr.lock.Unlock()
	s, ok := globalStorageMgr.storages[p]
	if !ok {
		var err error
		s, err = globalStorageMgr.Open(typ, p)
		if err != nil {
			return nil, err
		}
		globalStorageMgr.storages[p] = s
	}
	return s, nil
}
