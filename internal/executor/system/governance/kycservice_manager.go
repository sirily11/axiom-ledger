package governance

import (
	"encoding/json"
	"errors"
	"fmt"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"

	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-ledger/internal/executor/system/access"
	"github.com/axiomesh/axiom-ledger/internal/executor/system/common"
	"github.com/axiomesh/axiom-ledger/internal/ledger"
	vm "github.com/axiomesh/eth-kit/evm"
)

var (
	ErrNotFoundProposal = errors.New("not found this proposal")
)

const (
	KycProposalGas uint64 = 30000
	KycVoteGas     uint64 = 30000
	KycProposalKey        = "KycProposal"
)

var _ common.SystemContract = (*KycServiceManager)(nil)

type KycProposalArgs struct {
	BaseProposalArgs
	access.KycServiceArgs
}

type KycProposal struct {
	BaseProposal
	access.KycServiceArgs
}

type KycVoteArgs struct {
	BaseVoteArgs
}

type KycServiceManager struct {
	gov *Governance

	account        ledger.IAccount
	councilAccount ledger.IAccount
	stateLedger    ledger.StateLedger
	currentLog     *common.Log
	proposalID     *ProposalID
	lastHeight     uint64
}

func (ac *KycServiceManager) Reset(lastHeight uint64, stateLedger ledger.StateLedger) {
	addr := types.NewAddressByStr(common.KycServiceContractAddr)
	ac.account = stateLedger.GetOrCreateAccount(addr)
	ac.stateLedger = stateLedger
	ac.currentLog = &common.Log{
		Address: addr,
	}
	ac.proposalID = NewProposalID(stateLedger)

	councilAddr := types.NewAddressByStr(common.CouncilManagerContractAddr)
	ac.councilAccount = stateLedger.GetOrCreateAccount(councilAddr)

	ac.checkAndUpdateState(lastHeight)
	ac.lastHeight = lastHeight
}

// NewKycServiceManager constructs a new NewKycServiceManager
func NewKycServiceManager(cfg *common.SystemContractConfig) *KycServiceManager {
	gov, err := NewGov([]ProposalType{KycServiceAdd, KycServiceRemove}, cfg.Logger)
	if err != nil {
		panic(err)
	}

	return &KycServiceManager{
		gov: gov,
	}
}

func (ac *KycServiceManager) Run(msg *vm.Message) (*vm.ExecutionResult, error) {
	defer ac.gov.SaveLog(ac.stateLedger, ac.currentLog)

	// parse method and arguments from msg payload
	args, err := ac.gov.GetArgs(msg)
	if err != nil {
		return nil, err
	}

	switch v := args.(type) {
	case *ProposalArgs:
		proposalArgs, err := ac.getKycProposalArgs(v)
		if err != nil {
			return nil, err
		}
		return ac.propose(&msg.From, proposalArgs)
	case *VoteArgs:
		return ac.vote(&msg.From, &KycVoteArgs{BaseVoteArgs: v.BaseVoteArgs})
	case *GetProposalArgs:
		return ac.getProposal(v.ProposalID)
	default:
		return nil, errors.New("unknown proposal args")
	}
}

func (ac *KycServiceManager) EstimateGas(callArgs *types.CallArgs) (uint64, error) {
	args, err := ac.gov.GetArgs(&vm.Message{Data: *callArgs.Data})
	if err != nil {
		return 0, err
	}

	var gas uint64
	switch args.(type) {
	case *ProposalArgs:
		gas = KycProposalGas
	case *VoteArgs:
		gas = KycVoteGas
	default:
		return 0, errors.New("unknown proposal args")
	}

	return gas, nil
}

func (ac *KycServiceManager) checkAndUpdateState(lastHeight uint64) {
	// TODO: need use CheckAndUpdate
}

func (ac *KycServiceManager) vote(user *ethcommon.Address, voteArgs *KycVoteArgs) (*vm.ExecutionResult, error) {
	result := &vm.ExecutionResult{UsedGas: KycVoteGas}

	// get proposal
	proposal, err := ac.loadKycProposal(voteArgs.ProposalId)
	if err != nil {
		return nil, err
	}

	result.ReturnData, result.Err = ac.voteServicesAddRemove(user, proposal, voteArgs)
	if result.Err != nil {
		return nil, result.Err
	}
	return result, nil
}

func (c *KycServiceManager) loadKycProposal(proposalID uint64) (*KycProposal, error) {
	isExist, data := c.account.GetState([]byte(fmt.Sprintf("%s%d", KycProposalKey, proposalID)))
	if !isExist {
		return nil, errors.New("node proposal not found for the id")
	}

	proposal := &KycProposal{}
	if err := json.Unmarshal(data, proposal); err != nil {
		return nil, err
	}

	return proposal, nil
}

func (ac *KycServiceManager) voteServicesAddRemove(user *ethcommon.Address, proposal *KycProposal, voteArgs *KycVoteArgs) ([]byte, error) {
	// check user can vote
	isExist, _ := CheckInCouncil(ac.councilAccount, user.String())
	if !isExist {
		return nil, ErrNotFoundCouncilMember
	}

	res := VoteResult(voteArgs.VoteResult)
	proposalStatus, err := ac.gov.Vote(user, &proposal.BaseProposal, res)
	if err != nil {
		return nil, err
	}
	proposal.Status = proposalStatus

	b, err := ac.saveKycProposal(proposal)
	if err != nil {
		return nil, err
	}

	// if proposal is approved, update the node members
	if proposal.Status == Approved {
		modifyType := access.AddKycService
		if proposal.Type == KycServiceRemove {
			modifyType = access.RemoveKycService
		}
		err = access.AddAndRemoveKycService(ac.stateLedger, modifyType, proposal.Services)
		if err != nil {
			return nil, err
		}
	}
	// record log
	ac.gov.RecordLog(ac.currentLog, VoteMethod, &proposal.BaseProposal, b)
	// vote not return value
	return nil, nil
}

func (ac *KycServiceManager) propose(addr *ethcommon.Address, args *KycProposalArgs) (*vm.ExecutionResult, error) {
	result := &vm.ExecutionResult{
		UsedGas: KycProposalGas,
	}

	result.ReturnData, result.Err = ac.proposeServicesAddRemove(addr, args)
	if result.Err != nil {
		return nil, result.Err
	}
	return result, nil
}

func (ac *KycServiceManager) getKycProposalArgs(args *ProposalArgs) (*KycProposalArgs, error) {
	kycArgs := &KycProposalArgs{
		BaseProposalArgs: args.BaseProposalArgs,
	}
	serviceArgs := &access.KycServiceArgs{}
	if err := json.Unmarshal(args.Extra, serviceArgs); err != nil {
		return nil, errors.New("unmarshal node extra arguments error")
	}
	kycArgs.KycServiceArgs = *serviceArgs
	return kycArgs, nil
}

// getProposal view proposal details
func (ac *KycServiceManager) getProposal(proposalID uint64) (*vm.ExecutionResult, error) {
	result := &vm.ExecutionResult{}

	isExist, b := ac.account.GetState([]byte(fmt.Sprintf("%s%d", KycProposalKey, proposalID)))
	if isExist {
		packed, err := ac.gov.PackOutputArgs(ProposalMethod, b)
		if err != nil {
			return nil, err
		}
		result.ReturnData = packed
		return result, nil
	}
	return nil, ErrNotFoundProposal
}

func (ac *KycServiceManager) proposeServicesAddRemove(addr *ethcommon.Address, args *KycProposalArgs) ([]byte, error) {
	baseProposal, err := ac.gov.Propose(addr, ProposalType(args.ProposalType), args.Title, args.Desc, args.BlockNumber, ac.lastHeight)
	if err != nil {
		return nil, err
	}
	isExist, council := CheckInCouncil(ac.councilAccount, addr.String())
	if !isExist {
		return nil, ErrNotFoundCouncilMember
	}
	proposal := &KycProposal{
		BaseProposal: *baseProposal,
	}
	id, err := ac.proposalID.GetAndAddID()
	if err != nil {
		return nil, err
	}
	proposal.ID = id
	proposal.Services = args.Services
	proposal.TotalVotes = lo.Sum[uint64](lo.Map[*CouncilMember, uint64](council.Members, func(item *CouncilMember, index int) uint64 {
		return item.Weight
	}))
	b, err := ac.saveKycProposal(proposal)
	if err != nil {
		return nil, err
	}
	returnData, err := ac.gov.PackOutputArgs(ProposeMethod, id)
	if err != nil {
		return nil, err
	}
	// record log
	ac.gov.RecordLog(ac.currentLog, ProposeMethod, &proposal.BaseProposal, b)
	// propose return id
	return returnData, nil
}

func (ac *KycServiceManager) saveKycProposal(proposal *KycProposal) ([]byte, error) {
	b, err := json.Marshal(proposal)
	if err != nil {
		return nil, err
	}
	// save proposal
	ac.account.SetState([]byte(fmt.Sprintf("%s%d", KycProposalKey, proposal.ID)), b)
	return b, nil
}