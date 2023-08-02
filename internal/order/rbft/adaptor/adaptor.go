package adaptor

import (
	"context"
	"crypto/ecdsa"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/axiomesh/axiom-bft/external"
	rbfttypes "github.com/axiomesh/axiom-bft/types"
	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-kit/types/pb"
	"github.com/axiomesh/axiom/internal/order"
	"github.com/axiomesh/axiom/internal/peermgr"
)

var _ external.ExternalStack[types.Transaction, *types.Transaction] = (*RBFTAdaptor)(nil)
var _ external.Storage = (*RBFTAdaptor)(nil)
var _ external.Network = (*RBFTAdaptor)(nil)
var _ external.Crypto = (*RBFTAdaptor)(nil)
var _ external.ServiceOutbound = (*RBFTAdaptor)(nil)
var _ external.EpochService = (*RBFTAdaptor)(nil)

type RBFTAdaptor struct {
	localID           uint64
	store             *storageWrapper
	peerMgr           peermgr.OrderPeerManager
	priv              *ecdsa.PrivateKey
	Nodes             map[uint64]*types.VpInfo
	nodePIDToID       map[string]uint64
	ReadyC            chan *Ready
	BlockC            chan *types.CommitEvent
	logger            logrus.FieldLogger
	getChainMetaFunc  func() *types.ChainMeta
	StateUpdating     bool
	StateUpdateHeight uint64
	applyConfChange   func(cc *rbfttypes.ConfState)
	cancel            context.CancelFunc
	isNew             bool
	config            *order.Config
}

type Ready struct {
	TXs       []*types.Transaction
	LocalList []bool
	Height    uint64
	Timestamp int64
}

func NewRBFTAdaptor(config *order.Config, blockC chan *types.CommitEvent, cancel context.CancelFunc) (*RBFTAdaptor, error) {
	store, err := newStorageWrapper(config.StoragePath, config.StorageType)
	if err != nil {
		return nil, err
	}

	nodePIDToID := make(map[string]uint64)
	for k, v := range config.Nodes {
		nodePIDToID[v.Pid] = k
	}
	stack := &RBFTAdaptor{
		localID:          config.ID,
		store:            store,
		peerMgr:          config.PeerMgr,
		priv:             config.PrivKey,
		Nodes:            config.Nodes,
		nodePIDToID:      nodePIDToID,
		ReadyC:           make(chan *Ready, 1024),
		logger:           config.Logger,
		getChainMetaFunc: config.GetChainMetaFunc,
		BlockC:           blockC,
		cancel:           cancel,
		isNew:            config.IsNew,
		config:           config,
	}

	return stack, nil
}

func (s *RBFTAdaptor) SetApplyConfChange(applyConfChange func(cc *rbfttypes.ConfState)) {
	s.applyConfChange = applyConfChange
}

func (s *RBFTAdaptor) getBlock(id uint64, i int) (*types.Block, error) {
	m := &pb.Message{
		Type: pb.Message_GET_BLOCK,
		Data: []byte(strconv.Itoa(i)),
	}

	res, err := s.peerMgr.Send(id, m)
	if err != nil {
		return nil, err
	}

	block := &types.Block{}
	if err := block.Unmarshal(res.Data); err != nil {
		return nil, err
	}

	return block, nil
}
