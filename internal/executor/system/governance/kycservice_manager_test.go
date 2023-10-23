package governance

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/axiomesh/axiom-kit/storage/leveldb"
	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-ledger/internal/executor/system/access"
	"github.com/axiomesh/axiom-ledger/internal/executor/system/common"
	"github.com/axiomesh/axiom-ledger/internal/ledger"
	"github.com/axiomesh/axiom-ledger/internal/ledger/mock_ledger"
	"github.com/axiomesh/axiom-ledger/pkg/repo"
	vm "github.com/axiomesh/eth-kit/evm"
)

const (
	KycService1 = "0x1000000000000000000000000000000000000001"
	KycService2 = "0x1000000000000000000000000000000000000002"
)

type TestKycServiceProposal struct {
	ID          uint64
	Type        ProposalType
	Proposer    string
	TotalVotes  uint64
	PassVotes   []string
	RejectVotes []string
	Status      ProposalStatus
	Services    []*access.KycService
}

func TestKycServiceManager_RunForPropose(t *testing.T) {
	ks := NewKycServiceManager(&common.SystemContractConfig{
		Logger: logrus.New(),
	})

	mockCtl := gomock.NewController(t)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)

	accountCache, err := ledger.NewAccountCache()
	assert.Nil(t, err)
	repoRoot := t.TempDir()
	ld, err := leveldb.New(filepath.Join(repoRoot, "kycservice_manager"), nil)
	assert.Nil(t, err)
	account := ledger.NewAccount(ld, accountCache, types.NewAddressByStr(common.KycServiceContractAddr), ledger.NewChanger())

	stateLedger.EXPECT().GetOrCreateAccount(gomock.Any()).Return(account).AnyTimes()
	stateLedger.EXPECT().AddLog(gomock.Any()).AnyTimes()
	stateLedger.EXPECT().SetBalance(gomock.Any(), gomock.Any()).AnyTimes()

	err = InitCouncilMembers(stateLedger, []*repo.Admin{
		{
			Address: admin1,
			Weight:  1,
			Name:    "111",
		},
		{
			Address: admin2,
			Weight:  1,
			Name:    "222",
		},
		{
			Address: admin3,
			Weight:  1,
			Name:    "333",
		},
		{
			Address: admin4,
			Weight:  1,
			Name:    "444",
		},
	}, "10")
	assert.Nil(t, err)

	testcases := []struct {
		Caller   string
		Data     []byte
		Expected vm.ExecutionResult
		Err      error
	}{
		{
			Caller: KycService1,
			Data: generateKycProposeData(t, KycServiceAdd, access.KycServiceArgs{
				Services: []*access.KycService{
					{
						KycAddr: *types.NewAddressByStr(KycService1),
					},
					{
						KycAddr: *types.NewAddressByStr(KycService2),
					},
				},
			}),
			Expected: vm.ExecutionResult{},
			Err:      ErrNotFoundCouncilMember,
		},
		{
			Caller: admin1,
			Data: generateKycProposeData(t, KycServiceAdd, access.KycServiceArgs{
				Services: []*access.KycService{
					{
						KycAddr: *types.NewAddressByStr(KycService1),
					},
					{
						KycAddr: *types.NewAddressByStr(KycService2),
					},
				},
			}),
			Expected: vm.ExecutionResult{
				UsedGas:    KycProposalGas,
				ReturnData: generateKycReturnData(t, ks.gov, 1),
			},
			Err: nil,
		},
		{
			Caller: admin1,
			Data:   []byte{0, 1, 2, 3},
			Expected: vm.ExecutionResult{
				UsedGas:    KycProposalGas,
				ReturnData: nil,
				Err:        ErrMethodName,
			},
			Err: ErrMethodName,
		},
	}

	for _, test := range testcases {
		ks.Reset(1, stateLedger)

		result, err := ks.Run(&vm.Message{
			From: types.NewAddressByStr(test.Caller).ETHAddress(),
			Data: test.Data,
		})
		assert.Equal(t, test.Err, err)

		if result != nil {
			assert.Equal(t, test.Expected.Err, result.Err)
			assert.Equal(t, test.Expected.UsedGas, result.UsedGas)

			assert.EqualValues(t, test.Expected.ReturnData, result.ReturnData)
		}
	}
}

func TestKycServiceManager_RunForVoteAdd(t *testing.T) {
	ks := NewKycServiceManager(&common.SystemContractConfig{
		Logger: logrus.New(),
	})

	mockCtl := gomock.NewController(t)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)

	accountCache, err := ledger.NewAccountCache()
	assert.Nil(t, err)
	repoRoot := t.TempDir()
	assert.Nil(t, err)
	ld, err := leveldb.New(filepath.Join(repoRoot, "kycservice_manager"), nil)
	assert.Nil(t, err)
	account := ledger.NewAccount(ld, accountCache, types.NewAddressByStr(common.KycServiceContractAddr), ledger.NewChanger())

	stateLedger.EXPECT().GetOrCreateAccount(gomock.Any()).Return(account).AnyTimes()
	stateLedger.EXPECT().AddLog(gomock.Any()).AnyTimes()
	stateLedger.EXPECT().SetBalance(gomock.Any(), gomock.Any()).AnyTimes()

	err = InitCouncilMembers(stateLedger, []*repo.Admin{
		{
			Address: admin1,
			Weight:  1,
			Name:    "111",
		},
		{
			Address: admin2,
			Weight:  1,
			Name:    "222",
		},
		{
			Address: admin3,
			Weight:  1,
			Name:    "333",
		},
		{
			Address: admin4,
			Weight:  1,
			Name:    "444",
		},
	}, "10000000")
	assert.Nil(t, err)

	ks.Reset(1, stateLedger)

	addr := types.NewAddressByStr(admin1).ETHAddress()
	ks.propose(&addr, &KycProposalArgs{
		BaseProposalArgs: BaseProposalArgs{
			ProposalType: uint8(KycServiceAdd),
			Title:        "title",
			Desc:         "desc",
			BlockNumber:  1000,
		},
		KycServiceArgs: access.KycServiceArgs{
			Services: []*access.KycService{
				{
					KycAddr: *types.NewAddressByStr(KycService1),
				},
				{
					KycAddr: *types.NewAddressByStr(KycService2),
				},
			},
		},
	})

	testcases := []struct {
		Caller   string
		Data     []byte
		Expected vm.ExecutionResult
		Err      error
	}{
		{
			Caller: admin2,
			Data:   generateKycVoteData(t, ks.proposalID.GetID()-1, Pass),
			Expected: vm.ExecutionResult{
				UsedGas: KycVoteGas,
			},
			Err: nil,
		},
		{
			Caller: admin3,
			Data:   generateKycVoteData(t, ks.proposalID.GetID()-1, Pass),
			Expected: vm.ExecutionResult{
				UsedGas: KycVoteGas,
			},
			Err: nil,
		},
		{
			Caller: "0xfff0000000000000000000000000000000000000",
			Data:   generateKycVoteData(t, ks.proposalID.GetID()-1, Pass),
			Expected: vm.ExecutionResult{
				UsedGas: KycVoteGas,
				Err:     ErrNotFoundCouncilMember,
			},
			Err: ErrNotFoundCouncilMember,
		},
	}

	for _, test := range testcases {
		ks.Reset(1, stateLedger)

		result, err := ks.Run(&vm.Message{
			From: types.NewAddressByStr(test.Caller).ETHAddress(),
			Data: test.Data,
		})
		assert.Equal(t, test.Err, err)

		if result != nil {
			assert.Equal(t, nil, result.Err)
			assert.Equal(t, test.Expected.UsedGas, result.UsedGas)
		}
	}
}

func TestKycServiceManager_RunForVoteRemove(t *testing.T) {
	ks := NewKycServiceManager(&common.SystemContractConfig{
		Logger: logrus.New(),
	})

	mockCtl := gomock.NewController(t)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)

	accountCache, err := ledger.NewAccountCache()
	assert.Nil(t, err)
	repoRoot := t.TempDir()
	assert.Nil(t, err)
	ld, err := leveldb.New(filepath.Join(repoRoot, "kycservice_manager"), nil)
	assert.Nil(t, err)
	account := ledger.NewAccount(ld, accountCache, types.NewAddressByStr(common.KycServiceContractAddr), ledger.NewChanger())

	stateLedger.EXPECT().GetOrCreateAccount(gomock.Any()).Return(account).AnyTimes()
	stateLedger.EXPECT().AddLog(gomock.Any()).AnyTimes()
	stateLedger.EXPECT().SetBalance(gomock.Any(), gomock.Any()).AnyTimes()

	err = InitCouncilMembers(stateLedger, []*repo.Admin{
		{
			Address: admin1,
			Weight:  1,
			Name:    "111",
		},
		{
			Address: admin2,
			Weight:  1,
			Name:    "222",
		},
		{
			Address: admin3,
			Weight:  1,
			Name:    "333",
		},
		{
			Address: admin4,
			Weight:  1,
			Name:    "444",
		},
	}, "10000000")
	assert.Nil(t, err)

	ks.Reset(1, stateLedger)

	addr := types.NewAddressByStr(admin1).ETHAddress()
	ks.propose(&addr, &KycProposalArgs{
		BaseProposalArgs: BaseProposalArgs{
			ProposalType: uint8(KycServiceRemove),
			Title:        "title",
			Desc:         "desc",
			BlockNumber:  1000,
		},
		KycServiceArgs: access.KycServiceArgs{
			Services: []*access.KycService{
				{
					KycAddr: *types.NewAddressByStr(KycService1),
				},
				{
					KycAddr: *types.NewAddressByStr(KycService2),
				},
			},
		},
	})

	testcases := []struct {
		Caller   string
		Data     []byte
		Expected vm.ExecutionResult
		Err      error
	}{
		{
			Caller: admin2,
			Data:   generateKycVoteData(t, ks.proposalID.GetID()-1, Pass),
			Expected: vm.ExecutionResult{
				UsedGas: KycVoteGas,
			},
			Err: nil,
		},
		{
			Caller: admin3,
			Data:   generateKycVoteData(t, ks.proposalID.GetID()-1, Pass),
			Expected: vm.ExecutionResult{
				UsedGas: KycVoteGas,
			},
			Err: errors.New("ACCESS ERROR: remove kyc services from an empty list"),
		},
	}

	for _, test := range testcases {
		ks.Reset(1, stateLedger)

		result, err := ks.Run(&vm.Message{
			From: types.NewAddressByStr(test.Caller).ETHAddress(),
			Data: test.Data,
		})
		assert.Equal(t, test.Err, err)

		if result != nil {
			assert.Equal(t, nil, result.Err)
			assert.Equal(t, test.Expected.UsedGas, result.UsedGas)
		}
	}
}

func TestKycServiceManager_GetProposal(t *testing.T) {
	ks := NewKycServiceManager(&common.SystemContractConfig{
		Logger: logrus.New(),
	})

	mockCtl := gomock.NewController(t)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)

	accountCache, err := ledger.NewAccountCache()
	assert.Nil(t, err)
	repoRoot := t.TempDir()
	assert.Nil(t, err)
	ld, err := leveldb.New(filepath.Join(repoRoot, "kycservice_manager"), nil)
	assert.Nil(t, err)
	account := ledger.NewAccount(ld, accountCache, types.NewAddressByStr(common.KycServiceContractAddr), ledger.NewChanger())

	stateLedger.EXPECT().GetOrCreateAccount(gomock.Any()).Return(account).AnyTimes()
	stateLedger.EXPECT().AddLog(gomock.Any()).AnyTimes()
	stateLedger.EXPECT().SetBalance(gomock.Any(), gomock.Any()).AnyTimes()

	err = InitCouncilMembers(stateLedger, []*repo.Admin{
		{
			Address: admin1,
			Weight:  1,
			Name:    "111",
		},
		{
			Address: admin2,
			Weight:  1,
			Name:    "222",
		},
		{
			Address: admin3,
			Weight:  1,
			Name:    "333",
		},
		{
			Address: admin4,
			Weight:  1,
			Name:    "444",
		},
	}, "10000000")
	assert.Nil(t, err)

	ks.Reset(1, stateLedger)

	addr := types.NewAddressByStr(admin1).ETHAddress()
	ks.propose(&addr, &KycProposalArgs{
		BaseProposalArgs: BaseProposalArgs{
			ProposalType: uint8(KycServiceRemove),
			Title:        "title",
			Desc:         "desc",
			BlockNumber:  1000,
		},
		KycServiceArgs: access.KycServiceArgs{
			Services: []*access.KycService{
				{
					KycAddr: *types.NewAddressByStr(KycService1),
				},
				{
					KycAddr: *types.NewAddressByStr(KycService2),
				},
			},
		},
	})

	execResult, err := ks.Run(&vm.Message{
		From: types.NewAddressByStr(admin1).ETHAddress(),
		Data: generateProposalData(t, 1),
	})
	assert.Nil(t, err)
	ret, err := ks.gov.UnpackOutputArgs(ProposalMethod, execResult.ReturnData)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, len(ret))

	proposal := &KycProposal{}
	err = json.Unmarshal(ret[0].([]byte), proposal)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, proposal.ID)
	assert.Equal(t, "desc", proposal.Desc)
	assert.EqualValues(t, 1, len(proposal.PassVotes))
	assert.EqualValues(t, 0, len(proposal.RejectVotes))

	tempaddr := types.NewAddressByStr(admin2).ETHAddress()
	_, err = ks.vote(&tempaddr, &KycVoteArgs{
		BaseVoteArgs: BaseVoteArgs{
			ProposalId: 1,
			VoteResult: uint8(Pass),
		},
	})
	assert.Nil(t, err)
	execResult, err = ks.Run(&vm.Message{
		From: types.NewAddressByStr(admin1).ETHAddress(),
		Data: generateProposalData(t, 1),
	})
	assert.Nil(t, err)
	ret, err = ks.gov.UnpackOutputArgs(ProposalMethod, execResult.ReturnData)
	assert.Nil(t, err)

	proposal = &KycProposal{}
	err = json.Unmarshal(ret[0].([]byte), proposal)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, proposal.ID)
	assert.EqualValues(t, 2, len(proposal.PassVotes))
	assert.EqualValues(t, 0, len(proposal.RejectVotes))
}

func TestKycServiceManager_EstimateGas(t *testing.T) {
	ks := NewKycServiceManager(&common.SystemContractConfig{
		Logger: logrus.New(),
	})

	from := types.NewAddressByStr(admin1).ETHAddress()
	to := types.NewAddressByStr(common.KycServiceContractAddr).ETHAddress()
	data := hexutil.Bytes(generateKycProposeData(t, KycServiceAdd, access.KycServiceArgs{
		Services: []*access.KycService{
			{
				KycAddr: *types.NewAddressByStr(KycService1),
			},
			{
				KycAddr: *types.NewAddressByStr(KycService2),
			},
		},
	}))
	// test propose
	gas, err := ks.EstimateGas(&types.CallArgs{
		From: &from,
		To:   &to,
		Data: &data,
	})
	assert.Nil(t, err)
	assert.Equal(t, KycProposalGas, gas)

	// test vote
	data = hexutil.Bytes(generateKycVoteData(t, 1, Pass))
	gas, err = ks.EstimateGas(&types.CallArgs{
		From: &from,
		To:   &to,
		Data: &data,
	})
	assert.Nil(t, err)
	assert.Equal(t, KycVoteGas, gas)

	// test error args
	data = hexutil.Bytes([]byte{0, 1, 2, 3})
	gas, err = ks.EstimateGas(&types.CallArgs{
		From: &from,
		To:   &to,
		Data: &data,
	})
	assert.NotNil(t, err)
	assert.Equal(t, uint64(0), gas)
}

func generateKycProposeData(t *testing.T, proposalType ProposalType, extraArgs access.KycServiceArgs) []byte {
	gabi, err := GetABI()

	title := "title"
	desc := "desc"
	blockNumber := uint64(1000)
	extra, err := json.Marshal(extraArgs)
	assert.Nil(t, err)
	data, err := gabi.Pack(ProposeMethod, uint8(proposalType), title, desc, blockNumber, extra)
	assert.Nil(t, err)

	return data
}

func generateKycVoteData(t *testing.T, proposalID uint64, voteResult VoteResult) []byte {
	gabi, err := GetABI()

	data, err := gabi.Pack(VoteMethod, proposalID, voteResult, []byte(""))
	assert.Nil(t, err)

	return data
}

func generateKycReturnData(t *testing.T, gov *Governance, id uint64) []byte {
	b, err := gov.PackOutputArgs(ProposeMethod, id)
	assert.Nil(t, err)

	return b
}

func TestKycServiceManager_Reset(t *testing.T) {
	ks := NewKycServiceManager(&common.SystemContractConfig{
		Logger: logrus.New(),
	})
	mockCtl := gomock.NewController(t)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)

	accountCache, err := ledger.NewAccountCache()
	assert.Nil(t, err)
	repoRoot := t.TempDir()
	assert.Nil(t, err)
	ld, err := leveldb.New(filepath.Join(repoRoot, "kycservice_manager"), nil)
	assert.Nil(t, err)
	account := ledger.NewAccount(ld, accountCache, types.NewAddressByStr(common.KycServiceContractAddr), ledger.NewChanger())
	stateLedger.EXPECT().GetOrCreateAccount(gomock.Any()).Return(account).AnyTimes()
	ks.Reset(100, stateLedger)
}

func TestKycServiceManager_loadKycProposal(t *testing.T) {
	ks := NewKycServiceManager(&common.SystemContractConfig{
		Logger: logrus.New(),
	})

	mockCtl := gomock.NewController(t)
	stateLedger := mock_ledger.NewMockStateLedger(mockCtl)

	accountCache, err := ledger.NewAccountCache()
	assert.Nil(t, err)
	repoRoot := t.TempDir()
	assert.Nil(t, err)
	ld, err := leveldb.New(filepath.Join(repoRoot, "kycservice_manager"), nil)
	assert.Nil(t, err)
	account := ledger.NewAccount(ld, accountCache, types.NewAddressByStr(common.KycServiceContractAddr), ledger.NewChanger())

	stateLedger.EXPECT().GetOrCreateAccount(gomock.Any()).Return(account).AnyTimes()
	ks.Reset(1, stateLedger)
	_, err = ks.loadKycProposal(1)
	assert.Equal(t, errors.New("node proposal not found for the id"), err)
}
