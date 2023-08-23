package finance

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom/internal/ledger"
	"github.com/axiomesh/axiom/pkg/repo"
	"github.com/axiomesh/eth-kit/ledger/mock_ledger"
)

const repoRoot = "testdata"

func TestGetGasPrice(t *testing.T) {
	// Gas should be less than 5000
	GasPriceBySize(t, 100, 5000000000000, nil)
	// Gas should be larger than 5000
	GasPriceBySize(t, 400, 5000000000000, nil)
	// Gas should be equals to 5000
	GasPriceBySize(t, 250, 5000000000000, nil)
	// Gas touch the ceiling
	GasPriceBySize(t, 400, 10000000000000, nil)
	// Gas touch the floor
	GasPriceBySize(t, 100, 1000000000000, nil)
	// Txs too much error
	GasPriceBySize(t, 700, 5000000000000, ErrTxsOutOfRange)
	// parent gas out of range error
	GasPriceBySize(t, 100, 11000000000000, ErrGasOutOfRange)
	// parent gas out of range error
	GasPriceBySize(t, 100, 900000000000, ErrGasOutOfRange)
}

func GasPriceBySize(t *testing.T, size int, parentGasPrice int64, expectErr error) uint64 {
	mockCtl := gomock.NewController(t)
	chainLedger := mock_ledger.NewMockChainLedger(mockCtl)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)
	mockLedger := &ledger.Ledger{
		ChainLedger: chainLedger,
		StateLedger: stateLedger,
	}
	chainLedger.EXPECT().Close().AnyTimes()
	stateLedger.EXPECT().Close().AnyTimes()
	defer mockLedger.Close()
	chainMeta := &types.ChainMeta{
		Height: 1,
	}
	// mock block for ledger
	chainLedger.EXPECT().GetChainMeta().Return(chainMeta).AnyTimes()
	block := &types.Block{
		BlockHeader:  &types.BlockHeader{GasPrice: parentGasPrice},
		Transactions: []*types.Transaction{},
	}
	prepareTxs := func(size int) []*types.Transaction {
		txs := []*types.Transaction{}
		for i := 0; i < size; i++ {
			txs = append(txs, &types.Transaction{})
		}
		return txs
	}
	block.Transactions = prepareTxs(size)
	chainLedger.EXPECT().GetBlock(uint64(1)).Return(block, nil).AnyTimes()
	config := generateMockConfig(t)
	gasPrice := NewGas(config, mockLedger)
	gas, err := gasPrice.GetGasPrice()
	if expectErr != nil {
		assert.EqualError(t, err, expectErr.Error())
		return 0
	}
	assert.Nil(t, err)
	return checkResult(t, block, config, parentGasPrice, gas)
}

func generateMockConfig(t *testing.T) *repo.Repo {
	repo, err := repo.Default(t.TempDir())
	assert.Nil(t, err)
	return repo
}

func checkResult(t *testing.T, block *types.Block, config *repo.Repo, parentGasPrice int64, gas uint64) uint64 {
	percentage := 2 * (float64(len(block.Transactions)) - float64(config.OrderConfig.Mempool.BatchSize)/2) / float64(config.OrderConfig.Mempool.BatchSize)
	actualGas := uint64(float64(parentGasPrice) * (1 + percentage*config.Config.Genesis.GasChangeRate))
	if actualGas > config.Config.Genesis.MaxGasPrice {
		actualGas = config.Config.Genesis.MaxGasPrice
	}
	if actualGas < config.Config.Genesis.MinGasPrice {
		actualGas = config.Config.Genesis.MinGasPrice
	}
	assert.Equal(t, uint64(actualGas), gas, "Gas price is not correct")
	return gas
}