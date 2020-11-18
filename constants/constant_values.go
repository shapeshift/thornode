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
	NativeChainGasFee
	NewPoolCycle
	MinimumNodesForYggdrasil
	MinimumNodesForBFT
	DesiredValidatorSet
	AsgardSize
	ChurnInterval
	ChurnRetryInterval
	ValidatorsChangeWindow
	LeaveProcessPerBlockHeight
	BadValidatorRate
	OldValidatorRate
	LackOfObservationPenalty
	SigningTransactionPeriod
	DoubleSignMaxAge
	MinimumBondInRune
	FundMigrationInterval
	ArtificialRagnarokBlockHeight
	MaximumLiquidityRune
	StrictBondLiquidityRatio
	DefaultPoolStatus
	FailKeygenSlashPoints
	FailKeysignSlashPoints
	LiquidityLockUpBlocks
	ObserveSlashPoints
	ObservationDelayFlexibility
	YggFundLimit
	JailTimeKeygen
	JailTimeKeysign
	CliTxCost
)

var nameToString = map[ConstantName]string{
	EmissionCurve:                 "EmissionCurve",
	IncentiveCurve:                "IncentiveCurve",
	BlocksPerYear:                 "BlocksPerYear",
	OutboundTransactionFee:        "OutboundTransactionFee",
	NativeChainGasFee:             "NativeChainGasFee",
	NewPoolCycle:                  "NewPoolCycle",
	MinimumNodesForYggdrasil:      "MinimumNodesForYggdrasil",
	MinimumNodesForBFT:            "MinimumNodesForBFT",
	DesiredValidatorSet:           "DesiredValidatorSet",
	AsgardSize:                    "AsgardSize",
	ChurnInterval:                 "ChurnInterval",
	ChurnRetryInterval:            "ChurnRetryInterval",
	ValidatorsChangeWindow:        "ValidatorsChangeWindow",
	LeaveProcessPerBlockHeight:    "LeaveProcessPerBlockHeight",
	BadValidatorRate:              "BadValidatorRate",
	OldValidatorRate:              "OldValidatorRate",
	LackOfObservationPenalty:      "LackOfObservationPenalty",
	SigningTransactionPeriod:      "SigningTransactionPeriod",
	DoubleSignMaxAge:              "DoubleSignMaxAge",
	MinimumBondInRune:             "MinimumBondInRune",
	FundMigrationInterval:         "FundMigrationInterval",
	ArtificialRagnarokBlockHeight: "ArtificialRagnarokBlockHeight",
	MaximumLiquidityRune:          "MaximumLiquidityRune",
	StrictBondLiquidityRatio:      "StrictBondLiquidityRatio",
	DefaultPoolStatus:             "DefaultPoolStatus",
	FailKeygenSlashPoints:         "FailKeygenSlashPoints",
	FailKeysignSlashPoints:        "FailKeysignSlashPoints",
	LiquidityLockUpBlocks:         "LiquidityLockUpBlocks",
	ObserveSlashPoints:            "ObserveSlashPoints",
	ObservationDelayFlexibility:   "ObservationDelayFlexibility",
	YggFundLimit:                  "YggFundLimit",
	JailTimeKeygen:                "JailTimeKeygen",
	JailTimeKeysign:               "JailTimeKeysign",
	CliTxCost:                     "CliTxCost",
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
