package system

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-ledger/internal/executor/system/common"
	"github.com/axiomesh/axiom-ledger/internal/ledger"
	"github.com/axiomesh/axiom-ledger/internal/ledger/mock_ledger"
	"github.com/axiomesh/axiom-ledger/pkg/repo"
)

var systemContractAddrs = []string{
	common.NodeManagerContractAddr,
}

var notSystemContractAddrs = []string{
	"0x1000000000000000000000000000000000000000",
	"0x0340000000000000000000000000000000000000",
	"0x0200000000000000000000000000000000000000",
	"0xffddd00000000000000000000000000000000000",
}

func TestContract_GetSystemContract(t *testing.T) {
	Initialize(logrus.New())

	for _, addr := range systemContractAddrs {
		contract, ok := GetSystemContract(types.NewAddressByStr(addr))
		assert.True(t, ok)
		assert.NotNil(t, contract)
	}

	for _, addr := range notSystemContractAddrs {
		contract, ok := GetSystemContract(types.NewAddressByStr(addr))
		assert.False(t, ok)
		assert.Nil(t, contract)
	}

	// test nil address
	contract, ok := GetSystemContract(nil)
	assert.False(t, ok)
	assert.Nil(t, contract)

	// test empty address
	contract, ok = GetSystemContract(&types.Address{})
	assert.False(t, ok)
	assert.Nil(t, contract)
}

func TestContractInitGenesisData(t *testing.T) {
	mockCtl := gomock.NewController(t)
	chainLedger := mock_ledger.NewMockChainLedger(mockCtl)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)
	mockLedger := &ledger.Ledger{
		ChainLedger: chainLedger,
		StateLedger: stateLedger,
	}

	genesis := repo.DefaultConfig(false)

	account := ledger.NewMockAccount(2, types.NewAddressByStr(common.NodeManagerContractAddr))
	stateLedger.EXPECT().GetOrCreateAccount(gomock.Any()).Return(account).AnyTimes()
	stateLedger.EXPECT().SetBalance(gomock.Any(), gomock.Any()).AnyTimes()

	err := InitGenesisData(&genesis.Genesis, mockLedger.StateLedger)
	assert.Nil(t, err)
}
