package thorchain

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// TXTYPE:STATE1:STATE2:STATE3:FINALMEMO

type TxType uint8

const (
	TxUnknown TxType = iota
	TxAdd
	TxWithdraw
	TxSwap
	TxOutbound
	TxDonate
	TxBond
	TxUnbond
	TxLeave
	TxYggdrasilFund
	TxYggdrasilReturn
	TxReserve
	TxRefund
	TxMigrate
	TxRagnarok
	TxSwitch
	TxNoOp
	TxConsolidate
)

var stringToTxTypeMap = map[string]TxType{
	"add":         TxAdd,
	"+":           TxAdd,
	"withdraw":    TxWithdraw,
	"wd":          TxWithdraw,
	"-":           TxWithdraw,
	"swap":        TxSwap,
	"s":           TxSwap,
	"=":           TxSwap,
	"out":         TxOutbound,
	"donate":      TxDonate,
	"d":           TxDonate,
	"bond":        TxBond,
	"unbond":      TxUnbond,
	"leave":       TxLeave,
	"yggdrasil+":  TxYggdrasilFund,
	"yggdrasil-":  TxYggdrasilReturn,
	"reserve":     TxReserve,
	"refund":      TxRefund,
	"migrate":     TxMigrate,
	"ragnarok":    TxRagnarok,
	"switch":      TxSwitch,
	"noop":        TxNoOp,
	"consolidate": TxConsolidate,
}

var txToStringMap = map[TxType]string{
	TxAdd:             "add",
	TxWithdraw:        "withdraw",
	TxSwap:            "swap",
	TxOutbound:        "out",
	TxRefund:          "refund",
	TxDonate:          "donate",
	TxBond:            "bond",
	TxUnbond:          "unbond",
	TxLeave:           "leave",
	TxYggdrasilFund:   "yggdrasil+",
	TxYggdrasilReturn: "yggdrasil-",
	TxReserve:         "reserve",
	TxMigrate:         "migrate",
	TxRagnarok:        "ragnarok",
	TxSwitch:          "switch",
	TxNoOp:            "noop",
	TxConsolidate:     "consolidate",
}

// converts a string into a txType
func StringToTxType(s string) (TxType, error) {
	// THORNode can support Abbreviated MEMOs , usually it is only one character
	sl := strings.ToLower(s)
	if t, ok := stringToTxTypeMap[sl]; ok {
		return t, nil
	}
	return TxUnknown, fmt.Errorf("invalid tx type: %s", s)
}

func (tx TxType) IsInbound() bool {
	switch tx {
	case TxAdd, TxWithdraw, TxSwap, TxDonate, TxBond, TxUnbond, TxLeave, TxSwitch, TxReserve, TxNoOp:
		return true
	default:
		return false
	}
}

func (tx TxType) IsOutbound() bool {
	switch tx {
	case TxOutbound, TxRefund, TxRagnarok:
		return true
	default:
		return false
	}
}

func (tx TxType) IsInternal() bool {
	switch tx {
	case TxYggdrasilFund, TxYggdrasilReturn, TxMigrate, TxConsolidate:
		return true
	default:
		return false
	}
}

// HasOutbound whether the txtype might trigger outbound tx
func (tx TxType) HasOutbound() bool {
	switch tx {
	case TxAdd, TxBond, TxDonate, TxYggdrasilReturn, TxReserve, TxMigrate, TxRagnarok, TxSwitch:
		return false
	default:
		return true
	}
}

func (tx TxType) IsEmpty() bool {
	return tx == TxUnknown
}

// Check if two txTypes are the same
func (tx TxType) Equals(tx2 TxType) bool {
	return tx == tx2
}

// Converts a txType into a string
func (tx TxType) String() string {
	return txToStringMap[tx]
}

type Memo interface {
	IsType(tx TxType) bool
	GetType() TxType
	IsEmpty() bool
	IsInbound() bool
	IsOutbound() bool
	IsInternal() bool

	String() string
	GetAsset() common.Asset
	GetAmount() cosmos.Uint
	GetDestination() common.Address
	GetSlipLimit() cosmos.Uint
	GetTxID() common.TxID
	GetAccAddress() cosmos.AccAddress
	GetBlockHeight() int64
}

type MemoBase struct {
	TxType TxType
	Asset  common.Asset
}

func (m MemoBase) String() string                   { return "" }
func (m MemoBase) GetType() TxType                  { return m.TxType }
func (m MemoBase) IsType(tx TxType) bool            { return m.TxType.Equals(tx) }
func (m MemoBase) GetAsset() common.Asset           { return m.Asset }
func (m MemoBase) GetAmount() cosmos.Uint           { return cosmos.ZeroUint() }
func (m MemoBase) GetDestination() common.Address   { return "" }
func (m MemoBase) GetSlipLimit() cosmos.Uint        { return cosmos.ZeroUint() }
func (m MemoBase) GetTxID() common.TxID             { return "" }
func (m MemoBase) GetAccAddress() cosmos.AccAddress { return cosmos.AccAddress{} }
func (m MemoBase) GetBlockHeight() int64            { return 0 }
func (m MemoBase) IsOutbound() bool                 { return m.TxType.IsOutbound() }
func (m MemoBase) IsInbound() bool                  { return m.TxType.IsInbound() }
func (m MemoBase) IsInternal() bool                 { return m.TxType.IsInternal() }
func (m MemoBase) IsEmpty() bool                    { return m.TxType.IsEmpty() }

func ParseMemo(memo string) (mem Memo, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panicked parsing memo(%s), err: %s", memo, r)
		}
	}()
	mem = MemoBase{TxType: TxUnknown}
	if len(memo) == 0 {
		return mem, fmt.Errorf("memo can't be empty")
	}
	parts := strings.Split(memo, ":")
	tx, err := StringToTxType(parts[0])
	if err != nil {
		return mem, err
	}

	var asset common.Asset
	switch tx {
	case TxDonate, TxAdd, TxSwap, TxWithdraw:
		if len(parts) < 2 {
			return mem, fmt.Errorf("cannot parse given memo: length %d", len(parts))
		}
		asset, err = common.NewAsset(parts[1])
		if err != nil {
			return mem, err
		}
	}

	switch tx {
	case TxLeave:
		return ParseLeaveMemo(parts)
	case TxDonate:
		return NewDonateMemo(asset), nil
	case TxAdd:
		return ParseAddLiquidityMemo(asset, parts)
	case TxWithdraw:
		return ParseWithdrawLiquidityMemo(asset, parts)
	case TxSwap:
		return ParseSwapMemo(asset, parts)
	case TxOutbound:
		return ParseOutboundMemo(parts)
	case TxRefund:
		return ParseRefundMemo(parts)
	case TxBond:
		return ParseBondMemo(parts)
	case TxUnbond:
		return ParseUnbondMemo(parts)
	case TxYggdrasilFund:
		return ParseYggdrasilFundMemo(parts)
	case TxYggdrasilReturn:
		return ParseYggdrasilReturnMemo(parts)
	case TxReserve:
		return NewReserveMemo(), nil
	case TxMigrate:
		return ParseMigrateMemo(parts)
	case TxRagnarok:
		return ParseRagnarokMemo(parts)
	case TxSwitch:
		return ParseSwitchMemo(parts)
	case TxNoOp:
		return ParseNoOpMemo(parts)
	case TxConsolidate:
		return ParseConsolidateMemo(parts)
	default:
		return mem, fmt.Errorf("TxType not supported: %s", tx.String())
	}
}
