package access

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/axiomesh/axiom-ledger/pkg/loggers"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sirupsen/logrus"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"

	"github.com/axiomesh/axiom-kit/types"
	"github.com/axiomesh/axiom-ledger/internal/executor/system/common"
	"github.com/axiomesh/axiom-ledger/internal/ledger"
	vm "github.com/axiomesh/eth-kit/evm"
)

const (
	SubmitMethod   = "Submit"
	RemoveMethod   = "Remove"
	KycInfoKey     = "kycinfo"
	KycServicesKey = "kycservices"
)

var _ common.SystemContract = (*KycVerification)(nil)

var (
	ErrCheckSubmitInfo = errors.New("check submit info fail")
	ErrCheckRemoveInfo = errors.New("check submit info fail")
)

type KycFlag uint8

type ModifyType uint8

const (
	AddKycService    ModifyType = 4
	RemoveKycService ModifyType = 5
)

const (
	LongTermValidity int64 = -1
)

const (
	NotVerified KycFlag = iota
	Verified
)

type KycInfo struct {
	User    string
	KycAddr string
	KycFlag KycFlag
	Expires int64
}

type BaseExtraArgs struct {
	Extra []byte
}

type SubmitArgs struct {
	KycInfos []KycInfo
}

type RemoveArgs struct {
	Addresses []string
}

type KycService struct {
	KycAddr string
}

type KycServiceArgs struct {
	Services []KycService
}

var method2Sig = map[string]string{
	SubmitMethod: "Submit(bytes)",
	RemoveMethod: "Remove(bytes)",
}

type KycVerification struct {
	stateLedger ledger.StateLedger
	account     ledger.IAccount
	currentLog  *common.Log
	logger      logrus.FieldLogger
	method2Sig  map[string][]byte
	gabi        *abi.ABI
}

// NewKycVerification constructs a new KycVerification
func NewKycVerification(cfg *common.SystemContractConfig) *KycVerification {
	gabi, err := GetABI()
	if err != nil {
		panic(err)
	}

	return &KycVerification{
		logger:     loggers.Logger(loggers.Access),
		gabi:       gabi,
		method2Sig: initMethodSignature(),
	}
}

const jsondata = `
[
	{"type": "function", "name": "Submit", "inputs": [{"name": "KycInfos", "type": "bytes"}]},
	{"type": "function", "name": "Remove", "inputs": [{"name": "Addresses", "type": "bytes"}]}
]
`

// GetABI get system contract abi
func GetABI() (*abi.ABI, error) {
	gabi, err := abi.JSON(strings.NewReader(jsondata))
	if err != nil {
		return nil, err
	}
	return &gabi, nil
}

func initMethodSignature() map[string][]byte {
	m2sig := make(map[string][]byte)
	for methodName, methodSig := range method2Sig {
		m2sig[methodName] = crypto.Keccak256([]byte(methodSig))
	}
	return m2sig
}

func (c *KycVerification) Reset(stateLedger ledger.StateLedger) {
	addr := types.NewAddressByStr(common.KycVerifyContractAddr)
	c.account = stateLedger.GetOrCreateAccount(addr)
	c.stateLedger = stateLedger
	c.currentLog = &common.Log{
		Address: addr,
	}
}

func (c *KycVerification) EstimateGas(callArgs *types.CallArgs) (uint64, error) {
	_, err := c.getArgs(&vm.Message{Data: *callArgs.Data})
	if err != nil {
		return 0, err
	}
	dynamicGas := common.CalculateDynamicGas(*callArgs.Data)
	return dynamicGas, nil
}

func (c *KycVerification) CheckAndUpdateState(lastHeight uint64, stateLedger ledger.StateLedger) {}

func (c *KycVerification) Run(msg *vm.Message) (*vm.ExecutionResult, error) {
	defer c.SaveLog(c.stateLedger, c.currentLog)
	// parse method and arguments from msg payload
	args, err := c.getArgs(msg)
	if err != nil {
		return nil, err
	}
	var result *vm.ExecutionResult
	switch v := args.(type) {
	case *SubmitArgs:
		result, err = c.Submit(&msg.From, v)
	case *RemoveArgs:
		result, err = c.Remove(&msg.From, v)
	default:
		return nil, fmt.Errorf("access error: Run: unknown access args")
	}
	usedGas := common.CalculateDynamicGas(msg.Data)
	if result != nil {
		result.UsedGas = usedGas
	}
	return result, err
}

func (c *KycVerification) getArgs(msg *vm.Message) (any, error) {
	data := msg.Data
	if data == nil {
		return nil, vm.ErrExecutionReverted
	}
	method, err := c.getMethodName(data)
	if err != nil {
		return nil, err
	}
	switch method {
	case SubmitMethod:
		args := &BaseExtraArgs{}
		if err = c.ParseArgs(msg, SubmitMethod, args); err != nil {
			return nil, err
		}
		submitArgs := &SubmitArgs{}
		if err = json.Unmarshal(args.Extra, submitArgs); err != nil {
			return nil, err
		}
		return submitArgs, nil
	case RemoveMethod:
		args := &BaseExtraArgs{}
		if err = c.ParseArgs(msg, RemoveMethod, args); err != nil {
			return nil, err
		}
		removeArgs := &RemoveArgs{}
		if err = json.Unmarshal(args.Extra, removeArgs); err != nil {
			return nil, err
		}
		return removeArgs, nil
	default:
		return nil, fmt.Errorf("access error: getArgs: wrong method name")
	}
}

// ParseArgs parse the arguments to specified interface by method name
func (c *KycVerification) ParseArgs(msg *vm.Message, methodName string, ret any) error {
	if len(msg.Data) < 4 {
		return fmt.Errorf("access error: ParseArgs: msg data length is not improperly formatted: %q - Bytes: %+v", msg.Data, msg.Data)
	}
	// discard method id
	data := msg.Data[4:]
	var args abi.Arguments
	if method, ok := c.gabi.Methods[methodName]; ok {
		if len(data)%32 != 0 {
			return fmt.Errorf("access error: ParseArgs: improperly formatted output: %q - Bytes: %+v", data, data)
		}
		args = method.Inputs
	}
	if args == nil {
		return fmt.Errorf("access error: ParseArgs: could not locate named method: %s", methodName)
	}
	unpacked, err := args.Unpack(data)
	if err != nil {
		return err
	}
	return args.Copy(ret, unpacked)
}

func (c *KycVerification) getMethodName(data []byte) (string, error) {
	for methodName, methodSig := range c.method2Sig {
		id := methodSig[:4]
		c.logger.Debugf("access error: getMethodName: method is: %v, get method id: %v", id, data[:4])
		if bytes.Equal(id, data[:4]) {
			return methodName, nil
		}
	}
	return "", fmt.Errorf("access error: getMethodName")
}

func (c *KycVerification) Submit(from *ethcommon.Address, args *SubmitArgs) (*vm.ExecutionResult, error) {
	success := CheckInServices(c.account, from.String())
	if !success {
		return nil, fmt.Errorf("access error: Submit: fail by checking kyc services")
	}
	for _, info := range args.KycInfos {
		err := c.checkSubmitInfo(from, info)
		if err != nil {
			return nil, err
		}
		err = c.saveKycInfo(info)
		if err != nil {
			return nil, err
		}
	}

	b, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return &vm.ExecutionResult{
		ReturnData: b,
	}, nil
}

func (c *KycVerification) Remove(from *ethcommon.Address, args *RemoveArgs) (*vm.ExecutionResult, error) {
	success := CheckInServices(c.account, from.String())
	if !success {
		return nil, fmt.Errorf("access error: Remove: check kyc service fail")
	}
	for _, addr := range args.Addresses {
		err := c.checkRemoveInfo(addr)
		if err != nil {
			return nil, err
		}
		err = c.removeKycInfo(addr)
		if err != nil {
			return nil, err
		}
	}
	b, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return &vm.ExecutionResult{
		ReturnData: b,
	}, nil
}

func Verify(lg ledger.StateLedger, needApprove string) (bool, error) {
	logger := loggers.Logger(loggers.Access)
	account := lg.GetOrCreateAccount(types.NewAddressByStr(common.KycVerifyContractAddr))
	state, bytes := account.GetState([]byte(KycInfoKey + needApprove))
	if !state {
		logger.Debugf("verify user addr fail by GetState, addr is %s", needApprove)
		return false, fmt.Errorf("access error")
	}
	info := &KycInfo{}
	if err := json.Unmarshal(bytes, &info); err != nil {
		logger.Debugf("verify fail by json.Unmarshal, addr is %s", needApprove)
		return false, fmt.Errorf("access error")
	}
	// long-term validity
	if info.Expires == LongTermValidity && info.KycFlag == Verified {
		return true, nil
	}
	if time.Now().Unix() > info.Expires || info.KycFlag != Verified {
		logger.Debugf("verify fail by checking kyc info, addr is %s", needApprove)
		return false, fmt.Errorf("access error")
	}
	return true, nil
}

func (c *KycVerification) saveKycInfo(info KycInfo) error {
	b, err := json.Marshal(info)
	if err != nil {
		return err
	}
	c.account.SetState([]byte(KycInfoKey+info.User), b)
	return nil
}

func (c *KycVerification) getKycInfo(addr string) *KycInfo {
	state, i := c.account.GetState([]byte(KycInfoKey + addr))
	if state {
		res := &KycInfo{}
		err := json.Unmarshal(i, res)
		if err != nil {
			return res
		}
	}
	return nil
}

func (c *KycVerification) removeKycInfo(addr string) error {
	state, infoByte := c.account.GetState([]byte(KycInfoKey + addr))
	if !state {
		return nil
	}
	kycinfo := &KycInfo{}
	err := json.Unmarshal(infoByte, kycinfo)
	if err != nil {
		return err
	}
	kycinfo.KycFlag = NotVerified
	b, err := json.Marshal(kycinfo)
	if err != nil {
		return err
	}
	c.account.SetState([]byte(KycInfoKey+addr), b)
	return nil
}

func CheckInServices(account ledger.IAccount, addr string) bool {
	isExist, data := account.GetState([]byte(KycServicesKey))
	if !isExist {
		return false
	}
	var Services []*KycService
	if err := json.Unmarshal(data, &Services); err != nil {
		return false
	}
	if len(Services) == 0 {
		return false
	}
	isExist = common.IsInSlice[string](addr, lo.Map[*KycService, string](Services, func(item *KycService, index int) string {
		return item.KycAddr
	}))
	return isExist
}

func AddAndRemoveKycService(lg ledger.StateLedger, modifyType ModifyType, inputServices []KycService) error {
	existServices, err := GetKycServices(lg)
	if err != nil {
		return err
	}
	switch modifyType {
	case AddKycService:
		existServices = append(existServices, inputServices...)
		addrToServiceMap := lo.Associate(existServices, func(service KycService) (string, KycService) {
			return service.KycAddr, service
		})
		existServices = lo.MapToSlice(addrToServiceMap, func(key string, value KycService) KycService {
			return value
		})
		return SetKycService(lg, existServices)
	case RemoveKycService:
		if len(existServices) > 0 {
			addrToServiceMap := lo.Associate(existServices, func(service KycService) (string, KycService) {
				return service.KycAddr, service
			})
			filteredMembers := lo.Reject(inputServices, func(service KycService, _ int) bool {
				_, exists := addrToServiceMap[service.KycAddr]
				return exists
			})
			existServices = filteredMembers
			return SetKycService(lg, existServices)
		} else {
			return fmt.Errorf("access error: remove kyc services from an empty list")
		}
	default:
		return fmt.Errorf("access error: wrong submit type")
	}
}

func SetKycService(lg ledger.StateLedger, services []KycService) error {
	cb, err := json.Marshal(services)
	if err != nil {
		return err
	}
	lg.GetOrCreateAccount(types.NewAddressByStr(common.KycVerifyContractAddr)).SetState([]byte(KycServicesKey), cb)
	return nil
}

func GetKycServices(lg ledger.StateLedger) ([]KycService, error) {
	success, data := lg.GetOrCreateAccount(types.NewAddressByStr(common.KycVerifyContractAddr)).GetState([]byte(KycServicesKey))
	var services []KycService
	if success {
		if err := json.Unmarshal(data, &services); err != nil {
			return nil, err
		}
		return services, nil
	}
	return services, nil
}

func InitKycServicesAndKycInfos(lg ledger.StateLedger, initVerifiedUsers []string, initKycServices []string) error {
	account := lg.GetOrCreateAccount(types.NewAddressByStr(common.KycVerifyContractAddr))
	uniqueMap := make(map[string]struct{})
	for _, str := range initVerifiedUsers {
		uniqueMap[str] = struct{}{}
	}
	for _, str := range initKycServices {
		uniqueMap[str] = struct{}{}
	}
	allAddresses := make([]string, len(uniqueMap))
	i := 0
	for key := range uniqueMap {
		allAddresses[i] = key
		i++
	}
	sort.Strings(allAddresses)
	// set init verified users
	for _, addrStr := range allAddresses {
		info := &KycInfo{
			User:    addrStr,
			KycAddr: addrStr,
			KycFlag: Verified,
			Expires: LongTermValidity,
		}
		b, err := json.Marshal(info)
		if err != nil {
			return err
		}
		account.SetState([]byte(KycInfoKey+addrStr), b)
	}
	// set init kyc services addresses
	var kycServices []*KycService
	for _, addrStr := range initKycServices {
		service := &KycService{
			KycAddr: addrStr,
		}
		kycServices = append(kycServices, service)
	}
	kycServicesBytes, err := json.Marshal(kycServices)
	if err != nil {
		return err
	}
	account.SetState([]byte(KycServicesKey), kycServicesBytes)

	return nil
}

// SaveLog save log
func (c *KycVerification) SaveLog(stateLedger ledger.StateLedger, currentLog *common.Log) {
	if currentLog.Data != nil {
		stateLedger.AddLog(&types.EvmLog{
			Address: currentLog.Address,
			Topics:  currentLog.Topics,
			Data:    currentLog.Data,
			Removed: currentLog.Removed,
		})
	}
}

func (c *KycVerification) checkSubmitInfo(from *ethcommon.Address, info KycInfo) error {
	// check user addr
	if addr := types.NewAddressByStr(info.User); addr.ETHAddress().String() != info.User {
		c.logger.Debugf("access error: info user addr is invalid")
		return ErrCheckSubmitInfo
	}
	// check kyc service addr
	if info.KycAddr != from.String() {
		c.logger.Debugf("access error: kyc addr is not same")
		return ErrCheckSubmitInfo
	}
	// check time
	if time.Now().Unix() > info.Expires {
		c.logger.Debugf("access error: kyc info is expired")
		return ErrCheckSubmitInfo
	}
	// check kycflag
	if info.KycFlag != Verified {
		c.logger.Debugf("access error: kyc info is not verified")
		return ErrCheckSubmitInfo
	}
	// check admins
	kycInfo := c.getKycInfo(info.User)
	if kycInfo != nil && kycInfo.Expires == -1 && kycInfo.KycFlag == Verified {
		c.logger.Debugf("access error: try to modify system user's kyc info")
		return ErrCheckSubmitInfo
	}

	return nil
}

func (c *KycVerification) checkRemoveInfo(addr string) error {
	// check admin
	info := c.getKycInfo(addr)
	if info != nil && info.KycFlag == Verified && info.Expires == LongTermValidity {
		c.logger.Debugf("access error: try to remove system user's kyc info")
		return ErrCheckRemoveInfo
	}
	return nil
}
