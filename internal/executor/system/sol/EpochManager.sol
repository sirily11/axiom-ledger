// SPDX-License-Identifier: UNLICENSED

pragma solidity >=0.7.0 <0.9.0;

    struct ConsensusParams {
        string ValidatorElectionType;
        string ProposerElectionType;
        uint64 CheckpointPeriod;
        uint64 HighWatermarkCheckpointPeriod;
        uint64 MaxValidatorNum;
        uint64 BlockMaxTxNum;
        bool EnableTimedGenEmptyBlock;
        int64 NotActiveWeight;
        uint64 ExcludeView;
    }

    struct NodeInfo {
        uint64 ID;
        string AccountAddress;
        string P2PNodeID;
        int64 ConsensusVotingPower;
    }

    struct Finance {
        uint64 GasLimit;
        uint64 MaxGasPrice;
        uint64 MinGasPrice;
        uint64 GasChangeRateValue;
        uint64 GasChangeRateDecimals;
    }

    struct ConfigParams {
        uint64 TxMaxSize;
    }

    struct EpochInfo {
        uint64 Version;
        uint64 Epoch;
        uint64 EpochPeriod;
        uint64 StartBlock;
        string[] P2PBootstrapNodeAddresses;
        ConsensusParams ConsensusParams;
        NodeInfo[] CandidateSet;
        NodeInfo[] ValidatorSet;
        NodeInfo[] DataSyncerSet;
        Finance FinanceParams;
        ConfigParams ConfigParams;
    }

interface EpochManager {
    function currentEpoch() external view returns (EpochInfo memory epochInfo);

    function nextEpoch() external view returns (EpochInfo memory epochInfo);

    function historyEpoch(uint64 epochID)
    external
    view
    returns (EpochInfo memory epochInfo);
}
