package rbft

import (
	"time"

	"go.opentelemetry.io/otel/trace"

	rbft "github.com/axiomesh/axiom-bft"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	"github.com/axiomesh/axiom-bft/common/metrics/prometheus"
	"github.com/axiomesh/axiom-bft/txpool"
	rbfttypes "github.com/axiomesh/axiom-bft/types"
	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-ledger/internal/consensus/common"
	"github.com/axiomesh/axiom-ledger/pkg/loggers"
)

func defaultRbftConfig() rbft.Config {
	return rbft.Config{
		LastServiceState: &rbfttypes.ServiceState{
			MetaState: &rbfttypes.MetaState{
				Height: 0,
				Digest: "",
			},
			Epoch: 0,
		},
		SetSize:                   1000,
		BatchTimeout:              200 * time.Millisecond,
		RequestTimeout:            6 * time.Second,
		NullRequestTimeout:        9 * time.Second,
		VcResendTimeout:           10 * time.Second,
		CleanVCTimeout:            60 * time.Second,
		NewViewTimeout:            8 * time.Second,
		SyncStateTimeout:          1 * time.Second,
		SyncStateRestartTimeout:   10 * time.Second,
		FetchCheckpointTimeout:    5 * time.Second,
		FetchViewTimeout:          1 * time.Second,
		CheckPoolTimeout:          3 * time.Minute,
		FlowControl:               false,
		FlowControlMaxMem:         0,
		MetricsProv:               &disabled.Provider{},
		Tracer:                    trace.NewNoopTracerProvider().Tracer("axiom-ledger"),
		DelFlag:                   make(chan bool, 10),
		Logger:                    nil,
		NoTxBatchTimeout:          0,
		CheckPoolRemoveTimeout:    15 * time.Minute,
		CommittedBlockCacheNumber: 10,
	}
}

func generateRbftConfig(config *common.Config) (rbft.Config, txpool.Config, error) {
	readConfig := config.Config

	currentEpoch, err := config.GetCurrentEpochInfoFromEpochMgrContractFunc()
	if err != nil {
		return rbft.Config{}, txpool.Config{}, err
	}
	defaultConfig := defaultRbftConfig()
	defaultConfig.GenesisEpochInfo = config.GenesisEpochInfo
	defaultConfig.SelfAccountAddress = config.SelfAccountAddress
	defaultConfig.LastServiceState = &rbfttypes.ServiceState{
		MetaState: &rbfttypes.MetaState{
			Height: config.Applied,
			Digest: config.Digest,
		},
		Epoch: currentEpoch.Epoch,
	}
	defaultConfig.GenesisBlockDigest = config.GenesisDigest
	defaultConfig.Logger = &common.Logger{FieldLogger: config.Logger}

	if readConfig.Rbft.Timeout.Request > 0 {
		defaultConfig.RequestTimeout = readConfig.Rbft.Timeout.Request.ToDuration()
	}
	if readConfig.Rbft.Timeout.NullRequest > 0 {
		defaultConfig.NullRequestTimeout = readConfig.Rbft.Timeout.NullRequest.ToDuration()
	}
	if readConfig.Rbft.Timeout.ResendViewChange > 0 {
		defaultConfig.VcResendTimeout = readConfig.Rbft.Timeout.ResendViewChange.ToDuration()
	}
	if readConfig.Rbft.Timeout.CleanViewChange > 0 {
		defaultConfig.CleanVCTimeout = readConfig.Rbft.Timeout.CleanViewChange.ToDuration()
	}
	if readConfig.Rbft.Timeout.NewView > 0 {
		defaultConfig.NewViewTimeout = readConfig.Rbft.Timeout.NewView.ToDuration()
	}
	if readConfig.Rbft.Timeout.SyncState > 0 {
		defaultConfig.SyncStateTimeout = readConfig.Rbft.Timeout.SyncState.ToDuration()
	}
	if readConfig.Rbft.Timeout.SyncStateRestart > 0 {
		defaultConfig.SyncStateRestartTimeout = readConfig.Rbft.Timeout.SyncStateRestart.ToDuration()
	}
	if readConfig.Rbft.Timeout.FetchCheckpoint > 0 {
		defaultConfig.FetchCheckpointTimeout = readConfig.Rbft.Timeout.FetchCheckpoint.ToDuration()
	}
	if readConfig.Rbft.Timeout.FetchView > 0 {
		defaultConfig.FetchViewTimeout = readConfig.Rbft.Timeout.FetchView.ToDuration()
	}

	// txpool
	if readConfig.TxPool.BatchTimeout > 0 {
		defaultConfig.BatchTimeout = readConfig.TxPool.BatchTimeout.ToDuration()
	}
	if readConfig.TxPool.ToleranceTime > 0 {
		defaultConfig.CheckPoolTimeout = readConfig.TxPool.ToleranceTime.ToDuration()
	}
	if readConfig.TxCache.SetSize > 0 {
		defaultConfig.SetSize = readConfig.TxCache.SetSize
	}
	if readConfig.TimedGenBlock.NoTxBatchTimeout > 0 {
		defaultConfig.NoTxBatchTimeout = readConfig.TimedGenBlock.NoTxBatchTimeout.ToDuration()
	}
	if readConfig.TxPool.ToleranceRemoveTime > 0 {
		defaultConfig.CheckPoolRemoveTimeout = readConfig.TxPool.ToleranceRemoveTime.ToDuration()
	}
	if readConfig.Rbft.EnableMetrics {
		defaultConfig.MetricsProv = &prometheus.Provider{
			Name: "rbft",
		}
	}

	if readConfig.Rbft.CommittedBlockCacheNumber > 0 {
		defaultConfig.CommittedBlockCacheNumber = readConfig.Rbft.CommittedBlockCacheNumber
	}
	fn := func(addr string) uint64 {
		return config.GetAccountNonce(types.NewAddressByStr(addr))
	}
	txpoolConf := txpool.Config{
		Logger:              &common.Logger{FieldLogger: loggers.Logger(loggers.TxPool)},
		BatchSize:           defaultConfig.GenesisEpochInfo.ConsensusParams.BlockMaxTxNum,
		PoolSize:            readConfig.TxPool.PoolSize,
		ToleranceTime:       readConfig.TxPool.ToleranceTime.ToDuration(),
		ToleranceRemoveTime: readConfig.TxPool.ToleranceRemoveTime.ToDuration(),
		ToleranceNonceGap:   readConfig.TxPool.ToleranceNonceGap,
		GetAccountNonce:     fn,
		IsTimed:             defaultConfig.GenesisEpochInfo.ConsensusParams.EnableTimedGenEmptyBlock,
	}
	return defaultConfig, txpoolConf, nil
}
