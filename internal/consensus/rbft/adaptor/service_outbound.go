package adaptor

import (
	"errors"
	"fmt"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/gammazero/workerpool"
	"github.com/sirupsen/logrus"

	"github.com/axiomesh/axiom-bft/common/consensus"
	rbfttypes "github.com/axiomesh/axiom-bft/types"
	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-ledger/internal/consensus/common"
)

const (
	maxSyncBlockCacheSize = 10000
)

func (a *RBFTAdaptor) Execute(requests []*types.Transaction, localList []bool, seqNo uint64, timestamp int64, proposerAccount string) {
	a.ReadyC <- &Ready{
		Txs:             requests,
		LocalList:       localList,
		Height:          seqNo,
		Timestamp:       timestamp,
		ProposerAccount: proposerAccount,
	}
}

func (a *RBFTAdaptor) StateUpdate(lowWatermark, seqNo uint64, digest string, checkpoints []*consensus.SignedCheckpoint, epochChanges ...*consensus.EpochChange) {
	a.StateUpdating = true
	a.StateUpdateHeight = seqNo

	var peers []string

	// get the validator set of the remote latest epoch
	if len(epochChanges) != 0 {
		peers = epochChanges[len(epochChanges)-1].GetValidators()
	}

	for _, v := range a.EpochInfo.ValidatorSet {
		if v.AccountAddress != a.config.SelfAccountAddress {
			peers = append(peers, v.P2PNodeID)
		}
	}

	chain := a.getChainMetaFunc()

	startHeight := chain.Height + 1

	if chain.Height >= seqNo {
		localBlock, err := a.getBlockFunc(seqNo)
		if err != nil {
			panic("get local block failed")
		}
		if localBlock.BlockHash.String() != digest {
			a.logger.WithFields(logrus.Fields{
				"remote": digest,
				"local":  localBlock.BlockHash.String(),
				"height": seqNo,
			}).Warningf("Block hash is inconsistent in state update state, we need rollback")
			// rollback to the lowWatermark height
			startHeight = lowWatermark + 1
		} else {
			a.logger.WithFields(logrus.Fields{
				"remote": digest,
				"local":  localBlock.BlockHash.String(),
				"height": seqNo,
			}).Info("state update is ignored, because we have the same block")
			a.StateUpdating = false
			return
		}
	}

	// update the current sync height
	a.currentSyncHeight = startHeight

	a.logger.WithFields(logrus.Fields{
		"target":      a.StateUpdateHeight,
		"target_hash": digest,
		"start":       startHeight,
	}).Info("State update start")

	// todo(lrx): verify sign of each checkpoint?
	var stateUpdatedCheckpoint *consensus.Checkpoint
	if len(checkpoints) != 0 {
		stateUpdatedCheckpoint = checkpoints[0].GetCheckpoint()
	}

	syncSize := a.StateUpdateHeight - startHeight + 1
	fetchSizeLimit := a.config.Config.Sync.FetchSizeLimit
	if a.config.Config.Sync.FetchSizeLimit > maxSyncBlockCacheSize {
		fetchSizeLimit = maxSyncBlockCacheSize
	}
	pageSize := fetchSizeLimit
	if int(syncSize) < fetchSizeLimit {
		pageSize = int(syncSize)
	}

	for a.StateUpdateHeight >= a.currentSyncHeight {
		a.logger.WithFields(logrus.Fields{
			"target":     a.StateUpdateHeight,
			"pageSize":   pageSize,
			"syncHeight": a.currentSyncHeight,
		}).Info("State update page sync Start")
		if err := a.pageSyncBlock(peers, pageSize, stateUpdatedCheckpoint); err != nil {
			a.logger.Error(err)
			panic(err)
		}

		// update the current sync height and page size
		a.currentSyncHeight += uint64(pageSize)

		if int(a.StateUpdateHeight-a.currentSyncHeight+1) < fetchSizeLimit {
			pageSize = int(a.StateUpdateHeight - a.currentSyncHeight + 1)
		} else {
			pageSize = fetchSizeLimit
		}

	}

	// reset the current sync height
	a.currentSyncHeight = 0

	a.logger.WithFields(logrus.Fields{
		"target":      seqNo,
		"target_hash": digest,
	}).Info("State update finished fetch blocks")
}

func (a *RBFTAdaptor) get(peers []string, i int) (block *types.Block, err error) {
	for _, id := range peers {
		block, err = a.getBlock(id, i)
		if err != nil {
			a.logger.Error(err)
			continue
		}
		return block, nil
	}

	return nil, errors.New("can't get block from all peers")
}

func (a *RBFTAdaptor) pageSyncBlock(peers []string, pageSize int, checkpoints *consensus.Checkpoint) error {
	blockCache := a.getBlockFromOthers(peers, pageSize, a.currentSyncHeight)
	if len(blockCache) != pageSize && !a.closed {
		return fmt.Errorf("block cache size is not equal to page size, need %d, got %d", pageSize, len(blockCache))
	}

	for index, block := range blockCache {
		select {
		case <-a.ctx.Done():
			a.closed = true
			a.StateUpdating = false
			a.logger.Info("receive stop ctx, exist sync")
			return nil
		default:
			if block == nil {
				return fmt.Errorf("receive a nil block[height: %d]", int(a.currentSyncHeight)+index)
			}

			commitEvent := &common.CommitEvent{
				Block: block,
			}

			// if the last block is received, we should notify the state updated checkpoint to executor
			if blockCache[len(blockCache)-1].Height() == a.StateUpdateHeight {
				commitEvent.StateUpdatedCheckpoint = checkpoints
			}

			a.BlockC <- commitEvent
		}

	}
	return nil
}

func (a *RBFTAdaptor) getBlockFromOthers(peers []string, size int, currentSyncHeight uint64) []*types.Block {
	blockCache := make([]*types.Block, size)
	wp := workerpool.New(a.config.Config.Sync.FetchConcurrencyLimit)
	for i := 0; i < size; i++ {
		select {
		case <-a.ctx.Done():
			wp.Stop()
			a.closed = true
			a.StateUpdating = false
			a.logger.Info("receive stop ctx, exist sync")
			return nil
		default:
			index := i
			wp.Submit(func() {
				if err := retry.Retry(func(attempt uint) (err error) {
					curHeight := int(currentSyncHeight) + index
					block, err := a.get(peers, curHeight)
					if err != nil {
						a.logger.Info(err)
						return err
					}
					a.lock.Lock()
					blockCache[index] = block
					a.lock.Unlock()
					return nil
				}, strategy.Wait(500*time.Millisecond)); err != nil {
					a.logger.Error(err)
				}
			})
		}
	}
	wp.StopWait()
	return blockCache
}

func (a *RBFTAdaptor) SendFilterEvent(informType rbfttypes.InformType, message ...any) {
	// TODO: add implement
}

func (a *RBFTAdaptor) PostCommitEvent(commitEvent *common.CommitEvent) {
	a.postCommitEvent(commitEvent)
}

func (a *RBFTAdaptor) postCommitEvent(commitEvent *common.CommitEvent) {
	a.logger.WithFields(logrus.Fields{
		"height": commitEvent.Block.Height(),
		"hash":   commitEvent.Block.Hash().String(),
	}).Info("post commitEvent")
	a.BlockC <- commitEvent
}

func (a *RBFTAdaptor) GetCommitChannel() chan *common.CommitEvent {
	return a.BlockC
}
