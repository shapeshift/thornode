package thorchain

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type SwapMemo struct {
	MemoBase
	Destination          common.Address
	SlipLimit            cosmos.Uint
	AffiliateAddress     common.Address
	AffiliateBasisPoints cosmos.Uint
	DexAggregator        string
	DexTargetAddress     string
	DexTargetLimit       *cosmos.Uint
	OrderType            types.OrderType
	StreamFrequency      uint64
	StreamQuantity       uint64
}

func (m SwapMemo) GetDestination() common.Address       { return m.Destination }
func (m SwapMemo) GetSlipLimit() cosmos.Uint            { return m.SlipLimit }
func (m SwapMemo) GetAffiliateAddress() common.Address  { return m.AffiliateAddress }
func (m SwapMemo) GetAffiliateBasisPoints() cosmos.Uint { return m.AffiliateBasisPoints }
func (m SwapMemo) GetDexAggregator() string             { return m.DexAggregator }
func (m SwapMemo) GetDexTargetAddress() string          { return m.DexTargetAddress }
func (m SwapMemo) GetDexTargetLimit() *cosmos.Uint      { return m.DexTargetLimit }
func (m SwapMemo) GetOrderType() types.OrderType        { return m.OrderType }
func (m SwapMemo) GetStreamQuantity() uint64            { return m.StreamQuantity }
func (m SwapMemo) GetStreamFrequency() uint64           { return m.StreamFrequency }

func (m SwapMemo) String() string {
	slipLimit := m.SlipLimit.String()
	if m.SlipLimit.IsZero() {
		slipLimit = ""
	}
	if m.StreamFrequency > 0 || m.StreamQuantity > 0 {
		slipLimit = fmt.Sprintf("%s/%d/%d", m.SlipLimit.String(), m.StreamFrequency, m.StreamQuantity)
	}

	// prefer short notation for generate swap memo
	txType := m.TxType.String()
	if m.TxType == TxSwap {
		txType = "="
	}

	args := []string{
		txType,
		m.Asset.String(),
		m.Destination.String(),
		slipLimit,
		m.AffiliateAddress.String(),
		m.AffiliateBasisPoints.String(),
		m.DexAggregator,
		m.DexTargetAddress,
	}

	last := 3
	if !m.SlipLimit.IsZero() {
		last = 4
	}

	if !m.AffiliateAddress.IsEmpty() {
		last = 6
	}

	if m.DexAggregator != "" {
		last = 8
	}

	if m.DexTargetLimit != nil && !m.DexTargetLimit.IsZero() {
		args = append(args, m.DexTargetLimit.String())
		last = 9
	}

	return strings.Join(args[:last], ":")
}

func NewSwapMemo(asset common.Asset, dest common.Address, slip cosmos.Uint, affAddr common.Address, affPts cosmos.Uint, dexAgg, dexTargetAddress string, dexTargetLimit cosmos.Uint, orderType types.OrderType, quan, freq uint64) SwapMemo {
	swapMemo := SwapMemo{
		MemoBase:             MemoBase{TxType: TxSwap, Asset: asset},
		Destination:          dest,
		SlipLimit:            slip,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
		DexAggregator:        dexAgg,
		DexTargetAddress:     dexTargetAddress,
		OrderType:            orderType,
		StreamQuantity:       quan,
		StreamFrequency:      freq,
	}
	if !dexTargetLimit.IsZero() {
		swapMemo.DexTargetLimit = &dexTargetLimit
	}
	return swapMemo
}

func ParseSwapMemo(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (SwapMemo, error) {
	if keeper == nil {
		return ParseSwapMemoV1(ctx, keeper, asset, parts)
	}
	switch {
	case keeper.GetVersion().GTE(semver.MustParse("1.115.0")):
		return ParseSwapMemoV115(ctx, keeper, asset, parts)
	case keeper.GetVersion().GTE(semver.MustParse("1.112.0")):
		return ParseSwapMemoV112(ctx, keeper, asset, parts)
	case keeper.GetVersion().GTE(semver.MustParse("1.104.0")):
		return ParseSwapMemoV104(ctx, keeper, asset, parts)
	case keeper.GetVersion().GTE(semver.MustParse("1.98.0")):
		return ParseSwapMemoV98(ctx, keeper, asset, parts)
	case keeper.GetVersion().GTE(semver.MustParse("1.92.0")):
		return ParseSwapMemoV92(ctx, keeper, asset, parts)
	default:
		return ParseSwapMemoV1(ctx, keeper, asset, parts)
	}
}

func ParseSwapMemoV115(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (SwapMemo, error) {
	var err error
	var order types.OrderType
	dexAgg := ""
	dexTargetAddress := ""
	dexTargetLimit := cosmos.ZeroUint()
	if len(parts) < 2 {
		return SwapMemo{}, fmt.Errorf("not enough parameters")
	}
	// DESTADDR can be empty , if it is empty , it will swap to the sender address
	destination := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if strings.EqualFold(parts[0], "limito") || strings.EqualFold(parts[0], "lo") {
		order = types.OrderType_limit
	}
	if destStr := GetPart(parts, 2); destStr != "" {
		if keeper == nil {
			destination, err = common.NewAddress(destStr)
		} else {
			destination, err = FetchAddress(ctx, keeper, destStr, asset.Chain)
		}
		if err != nil {
			return SwapMemo{}, err
		}
	}

	// price limit can be empty , when it is empty , there is no price protection
	slip := cosmos.ZeroUint()
	streamFreq := uint64(0)
	streamQuantity := uint64(0)
	if limitStr := GetPart(parts, 3); limitStr != "" {
		if strings.Contains(limitStr, "/") {
			parts := strings.SplitN(limitStr, "/", 3)
			if len(parts) < 1 {
				return SwapMemo{}, fmt.Errorf("invalid streaming swap format: %s", limitStr)
			}
			slip, err = parseTradeTarget(parts[0])
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid: %s", parts[0], err)
			}
			if len(parts) > 1 {
				streamFreq, err = strconv.ParseUint(parts[1], 10, 64)
				if err != nil {
					return SwapMemo{}, fmt.Errorf("failed to parse stream frequency: %s: %s", parts[1], err)
				}
			}
			if len(parts) > 2 {
				streamQuantity, err = strconv.ParseUint(parts[2], 10, 64)
				if err != nil {
					return SwapMemo{}, fmt.Errorf("failed to parse stream quantity: %s: %s", parts[2], err)
				}
			}
		} else {
			slip, err = parseTradeTarget(limitStr)
			if err != nil {
				return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid: %s", limitStr, err)
			}
		}
	}

	affAddrStr := GetPart(parts, 4)
	affPtsStr := GetPart(parts, 5)
	if affAddrStr != "" && affPtsStr != "" {
		if keeper == nil {
			affAddr, err = common.NewAddress(affAddrStr)
		} else {
			affAddr, err = FetchAddress(ctx, keeper, affAddrStr, common.THORChain)
		}
		if err != nil {
			return SwapMemo{}, err
		}

		affPts, err = ParseAffiliateBasisPoints(ctx, keeper, affPtsStr)
		if err != nil {
			return SwapMemo{}, err
		}
	}

	dexAgg = GetPart(parts, 6)
	dexTargetAddress = GetPart(parts, 7)

	if x := GetPart(parts, 8); x != "" {
		dexTargetLimit, err = cosmos.ParseUint(x)
		if err != nil {
			ctx.Logger().Error("invalid dex target limit, ignore it", "limit", x)
			dexTargetLimit = cosmos.ZeroUint()
		}
	}

	return NewSwapMemo(asset, destination, slip, affAddr, affPts, dexAgg, dexTargetAddress, dexTargetLimit, order, streamQuantity, streamFreq), nil
}
