package constants

import (
	"fmt"

	"github.com/blang/semver"
)

// ConstantName the name we used to get constant values
type ConstantName int

const (
	EmissionCurve ConstantName = iota
	IncentiveCurve
	BlocksPerYear
	OutboundTransactionFee
	NativeTransactionFee
	KillSwitchStart
	KillSwitchDuration
	PoolCycle
	MinRunePoolDepth
	MaxAvailablePools
	StagedPoolCost
	MinimumNodesForYggdrasil
	MinimumNodesForBFT
	DesiredValidatorSet
	AsgardSize
	DerivedDepthBasisPts
	DerivedMinDepth
	MaxAnchorSlip
	MaxAnchorBlocks
	ChurnInterval
	ChurnRetryInterval
	ValidatorsChangeWindow
	LeaveProcessPerBlockHeight
	BadValidatorRedline
	BadValidatorRate
	OldValidatorRate
	LowBondValidatorRate
	LackOfObservationPenalty
	SigningTransactionPeriod
	DoubleSignMaxAge
	PauseBond
	PauseUnbond
	MinimumBondInRune
	FundMigrationInterval
	ArtificialRagnarokBlockHeight
	MaximumLiquidityRune
	StrictBondLiquidityRatio
	DefaultPoolStatus
	MaxOutboundAttempts
	SlashPenalty
	PauseOnSlashThreshold
	FailKeygenSlashPoints
	FailKeysignSlashPoints
	LiquidityLockUpBlocks
	ObserveSlashPoints
	ObservationDelayFlexibility
	YggFundLimit
	YggFundRetry
	JailTimeKeygen
	JailTimeKeysign
	NodePauseChainBlocks
	EnableDerivedAssets
	MinSwapsPerBlock
	MaxSwapsPerBlock
	EnableOrderBooks
	MaxSynthPerAssetDepth // TODO: remove me on hard fork
	MaxSynthPerPoolDepth
	VirtualMultSynths
	VirtualMultSynthsBasisPoints
	MinSlashPointsForBadValidator
	FullImpLossProtectionBlocks
	BondLockupPeriod
	MaxBondProviders
	NumberOfNewNodesPerChurn
	MinTxOutVolumeThreshold
	TxOutDelayRate
	TxOutDelayMax
	MaxTxOutOffset
	TNSRegisterFee
	TNSFeeOnSale
	TNSFeePerBlock
	PermittedSolvencyGap
	NodeOperatorFee
	ValidatorMaxRewardRatio
	PoolDepthForYggFundingMin
	MaxNodeToChurnOutForLowVersion
	POLMaxNetworkDeposit
	POLMaxPoolMovement
	POLSynthUtilization // TODO: remove me on hard fork
	POLTargetSynthPerPoolDepth
	POLBuffer
	RagnarokProcessNumOfLPPerIteration
	SwapOutDexAggregationDisabled
	SynthYieldBasisPoints
	SynthYieldCycle
	MinimumL1OutboundFeeUSD
	MinimumPoolLiquidityFee
	ILPCutoff
	ChurnMigrateRounds
	MaxAffiliateFeeBasisPoints
)

var nameToString = map[ConstantName]string{
	EmissionCurve:                      "EmissionCurve",
	IncentiveCurve:                     "IncentiveCurve",
	BlocksPerYear:                      "BlocksPerYear",
	OutboundTransactionFee:             "OutboundTransactionFee",
	NativeTransactionFee:               "NativeTransactionFee",
	PoolCycle:                          "PoolCycle",
	MinRunePoolDepth:                   "MinRunePoolDepth",
	MaxAvailablePools:                  "MaxAvailablePools",
	StagedPoolCost:                     "StagedPoolCost",
	KillSwitchStart:                    "KillSwitchStart",
	KillSwitchDuration:                 "KillSwitchDuration",
	MinimumNodesForYggdrasil:           "MinimumNodesForYggdrasil",
	MinimumNodesForBFT:                 "MinimumNodesForBFT",
	DesiredValidatorSet:                "DesiredValidatorSet",
	AsgardSize:                         "AsgardSize",
	DerivedDepthBasisPts:               "DerivedDepthBasisPts",
	DerivedMinDepth:                    "DerivedMinDepth",
	MaxAnchorSlip:                      "MaxAnchorSlip",
	MaxAnchorBlocks:                    "MaxAnchorBlocks",
	ChurnInterval:                      "ChurnInterval",
	ChurnRetryInterval:                 "ChurnRetryInterval",
	ValidatorsChangeWindow:             "ValidatorsChangeWindow",
	LeaveProcessPerBlockHeight:         "LeaveProcessPerBlockHeight",
	BadValidatorRedline:                "BadValidatorRedline",
	BadValidatorRate:                   "BadValidatorRate",
	OldValidatorRate:                   "OldValidatorRate",
	LowBondValidatorRate:               "LowBondValidatorRate",
	LackOfObservationPenalty:           "LackOfObservationPenalty",
	SigningTransactionPeriod:           "SigningTransactionPeriod",
	DoubleSignMaxAge:                   "DoubleSignMaxAge",
	PauseBond:                          "PauseBond",
	PauseUnbond:                        "PauseUnbond",
	MinimumBondInRune:                  "MinimumBondInRune",
	MaxBondProviders:                   "MaxBondProviders",
	FundMigrationInterval:              "FundMigrationInterval",
	ArtificialRagnarokBlockHeight:      "ArtificialRagnarokBlockHeight",
	MaximumLiquidityRune:               "MaximumLiquidityRune",
	StrictBondLiquidityRatio:           "StrictBondLiquidityRatio",
	DefaultPoolStatus:                  "DefaultPoolStatus",
	MaxOutboundAttempts:                "MaxOutboundAttempts",
	SlashPenalty:                       "SlashPenalty",
	PauseOnSlashThreshold:              "PauseOnSlashThreshold",
	FailKeygenSlashPoints:              "FailKeygenSlashPoints",
	FailKeysignSlashPoints:             "FailKeysignSlashPoints",
	LiquidityLockUpBlocks:              "LiquidityLockUpBlocks",
	ObserveSlashPoints:                 "ObserveSlashPoints",
	ObservationDelayFlexibility:        "ObservationDelayFlexibility",
	YggFundLimit:                       "YggFundLimit",
	YggFundRetry:                       "YggFundRetry",
	JailTimeKeygen:                     "JailTimeKeygen",
	JailTimeKeysign:                    "JailTimeKeysign",
	NodePauseChainBlocks:               "NodePauseChainBlocks",
	EnableDerivedAssets:                "EnableDerivedAssets",
	MinSwapsPerBlock:                   "MinSwapsPerBlock",
	MaxSwapsPerBlock:                   "MaxSwapsPerBlock",
	EnableOrderBooks:                   "EnableOrderBooks",
	VirtualMultSynths:                  "VirtualMultSynths",
	VirtualMultSynthsBasisPoints:       "VirtualMultSynthsBasisPoints",
	MaxSynthPerAssetDepth:              "MaxSynthPerAssetDepth", // TODO: remove me on hard fork
	MaxSynthPerPoolDepth:               "MaxSynthPerPoolDepth",
	MinSlashPointsForBadValidator:      "MinSlashPointsForBadValidator",
	FullImpLossProtectionBlocks:        "FullImpLossProtectionBlocks",
	BondLockupPeriod:                   "BondLockupPeriod",
	NumberOfNewNodesPerChurn:           "NumberOfNewNodesPerChurn",
	MinTxOutVolumeThreshold:            "MinTxOutVolumeThreshold",
	TxOutDelayRate:                     "TxOutDelayRate",
	TxOutDelayMax:                      "TxOutDelayMax",
	MaxTxOutOffset:                     "MaxTxOutOffset",
	TNSRegisterFee:                     "TNSRegisterFee",
	TNSFeeOnSale:                       "TNSFeeOnSale",
	TNSFeePerBlock:                     "TNSFeePerBlock",
	PermittedSolvencyGap:               "PermittedSolvencyGap",
	ValidatorMaxRewardRatio:            "ValidatorMaxRewardRatio",
	NodeOperatorFee:                    "NodeOperatorFee",
	PoolDepthForYggFundingMin:          "PoolDepthForYggFundingMin",
	MaxNodeToChurnOutForLowVersion:     "MaxNodeToChurnOutForLowVersion",
	SwapOutDexAggregationDisabled:      "SwapOutDexAggregationDisabled",
	POLMaxNetworkDeposit:               "POLMaxNetworkDeposit",
	POLMaxPoolMovement:                 "POLMaxPoolMovement",
	POLSynthUtilization:                "POLSynthUtilization", // TODO: remove me on hard fork
	POLTargetSynthPerPoolDepth:         "POLTargetSynthPerPoolDepth",
	POLBuffer:                          "POLBuffer",
	RagnarokProcessNumOfLPPerIteration: "RagnarokProcessNumOfLPPerIteration",
	SynthYieldBasisPoints:              "SynthYieldBasisPoints",
	SynthYieldCycle:                    "SynthYieldCycle",
	MinimumL1OutboundFeeUSD:            "MinimumL1OutboundFeeUSD",
	MinimumPoolLiquidityFee:            "MinimumPoolLiquidityFee",
	ILPCutoff:                          "ILPCutoff",
	ChurnMigrateRounds:                 "ChurnMigrateRounds",
	MaxAffiliateFeeBasisPoints:         "MaxAffiliateFeeBasisPoints",
}

// String implement fmt.stringer
func (cn ConstantName) String() string {
	val, ok := nameToString[cn]
	if !ok {
		return "NA"
	}
	return val
}

// ConstantValues define methods used to get constant values
type ConstantValues interface {
	fmt.Stringer
	GetInt64Value(name ConstantName) int64
	GetBoolValue(name ConstantName) bool
	GetStringValue(name ConstantName) string
}

// GetConstantValues will return an  implementation of ConstantValues which provide ways to get constant values
func GetConstantValues(ver semver.Version) ConstantValues {
	if ver.GTE(semver.MustParse("0.1.0")) {
		return NewConstantValue010()
	}
	return nil
}
