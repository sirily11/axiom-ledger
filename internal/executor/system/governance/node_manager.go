package governance

import (
	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom/internal/executor/system/common"
	vm "github.com/axiomesh/eth-kit/evm"
	"github.com/axiomesh/eth-kit/ledger"
	"github.com/sirupsen/logrus"
)

var _ common.SystemContract = (*NodeManager)(nil)

type NodeManager struct {
	gov *Governance

	account ledger.IAccount
}

func NewNodeManager(logger logrus.FieldLogger) *NodeManager {
	gov, err := NewGov([]ProposalType{NodeUpdate, NodeAdd, NodeRemove}, logger)
	if err != nil {
		panic(err)
	}

	return &NodeManager{
		gov: gov,
	}
}

func (nm *NodeManager) Reset(stateLedger ledger.StateLedger) {
	nm.account = stateLedger.GetOrCreateAccount(types.NewAddressByStr(common.NodeManagerContractAddr))
	globalProposalID = GetInstanceOfProposalID(stateLedger)
}

func (nm *NodeManager) Run(msg *vm.Message) (*vm.ExecutionResult, error) {
	// parse method and arguments from msg payload

	// TODO: execute, then generate proposal id

	return &vm.ExecutionResult{
		UsedGas:    0,
		Err:        nil,
		ReturnData: []byte{1},
	}, nil
}
