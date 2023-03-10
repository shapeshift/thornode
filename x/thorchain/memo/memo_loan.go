package thorchain

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// "LOAN+:BTC.BTC:bc1YYYYYY:minBTC:affAddr:affPts:dexAgg:dexTarAddr:DexTargetLimit"

type LoanOpenMemo struct {
	MemoBase
	TargetAsset          common.Asset
	TargetAddress        common.Address
	MinOut               cosmos.Uint
	AffiliateAddress     common.Address
	AffiliateBasisPoints cosmos.Uint
	DexAggregator        string
	DexTargetAddress     string
	DexTargetLimit       cosmos.Uint
}

func (m LoanOpenMemo) GetTargetAsset() common.Asset         { return m.TargetAsset }
func (m LoanOpenMemo) GetTargetAddress() common.Address     { return m.TargetAddress }
func (m LoanOpenMemo) GetMinOut() cosmos.Uint               { return m.MinOut }
func (m LoanOpenMemo) GetAffiliateAddress() common.Address  { return m.AffiliateAddress }
func (m LoanOpenMemo) GetAffiliateBasisPoints() cosmos.Uint { return m.AffiliateBasisPoints }
func (m LoanOpenMemo) GetDexAggregator() string             { return m.DexAggregator }
func (m LoanOpenMemo) GetDexTargetAddress() string          { return m.DexTargetAddress }

func NewLoanOpenMemo(targetAsset common.Asset, targetAddr common.Address, minOut cosmos.Uint, affAddr common.Address, affPts cosmos.Uint, dexAgg, dexTargetAddr string, dexTargetLimit cosmos.Uint) LoanOpenMemo {
	return LoanOpenMemo{
		MemoBase:             MemoBase{TxType: TxLoanOpen},
		TargetAsset:          targetAsset,
		TargetAddress:        targetAddr,
		MinOut:               minOut,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
		DexAggregator:        dexAgg,
		DexTargetAddress:     dexTargetAddr,
		DexTargetLimit:       dexTargetLimit,
	}
}

func ParseLoanOpenMemo(ctx cosmos.Context, keeper keeper.Keeper, targetAsset common.Asset, parts []string) (LoanOpenMemo, error) {
	var err error
	var targetAddress common.Address
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	minOut := cosmos.ZeroUint()
	var dexAgg, dexTargetAddr string
	dexTargetLimit := cosmos.ZeroUint()
	if len(parts) <= 2 {
		return LoanOpenMemo{}, fmt.Errorf("Not enough loan parameters")
	}

	if keeper == nil {
		targetAddress, err = common.NewAddress(parts[2])
	} else {
		targetAddress, err = FetchAddress(ctx, keeper, parts[2], targetAsset.GetChain())
	}
	if err != nil {
		return LoanOpenMemo{}, err
	}

	if len(parts) > 3 && len(parts[3]) > 0 {
		minOutUint, err := strconv.ParseUint(parts[3], 10, 64)
		if err != nil {
			return LoanOpenMemo{}, err
		}
		minOut = cosmos.NewUint(minOutUint)
	}

	if len(parts) > 5 && len(parts[4]) > 0 && len(parts[5]) > 0 {
		if keeper == nil {
			affAddr, err = common.NewAddress(parts[4])
		} else {
			affAddr, err = FetchAddress(ctx, keeper, parts[4], common.THORChain)
		}
		if err != nil {
			return LoanOpenMemo{}, err
		}
		pts, err := strconv.ParseUint(parts[5], 10, 64)
		if err != nil {
			return LoanOpenMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}

	if len(parts) > 6 && len(parts[6]) > 0 {
		dexAgg = parts[6]
	}

	if len(parts) > 7 && len(parts[7]) > 0 {
		dexTargetAddr = parts[7]
	}

	if len(parts) > 8 && len(parts[8]) > 0 {
		dexTargetLimit, err = cosmos.ParseUint(parts[8])
		if err != nil {
			if keeper != nil {
				ctx.Logger().Error("invalid dex target limit, ignore it", "limit", parts[8])
			}
			dexTargetLimit = cosmos.ZeroUint()
		}
	}

	return NewLoanOpenMemo(targetAsset, targetAddress, minOut, affAddr, affPts, dexAgg, dexTargetAddr, dexTargetLimit), nil
}

// "LOAN-:BTC.BTC:bc1YYYYYY:minOut"

type LoanRepaymentMemo struct {
	MemoBase
	Owner  common.Address
	MinOut cosmos.Uint
}

func NewLoanRepaymentMemo(asset common.Asset, owner common.Address, minOut cosmos.Uint) LoanRepaymentMemo {
	return LoanRepaymentMemo{
		MemoBase: MemoBase{TxType: TxLoanRepayment, Asset: asset},
		Owner:    owner,
		MinOut:   minOut,
	}
}

func ParseLoanRepaymentMemo(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (LoanRepaymentMemo, error) {
	var err error
	var owner common.Address
	minOut := cosmos.ZeroUint()
	if len(parts) <= 2 {
		return LoanRepaymentMemo{}, fmt.Errorf("Not enough loan parameters")
	}

	if keeper == nil {
		owner, err = common.NewAddress(parts[2])
	} else {
		owner, err = FetchAddress(ctx, keeper, parts[2], asset.Chain)
	}
	if err != nil {
		return LoanRepaymentMemo{}, err
	}

	if len(parts) > 3 && len(parts[3]) > 0 {
		min, err := strconv.ParseUint(parts[3], 10, 64)
		if err != nil {
			return LoanRepaymentMemo{}, err
		}
		minOut = cosmos.NewUint(min)
	}

	return NewLoanRepaymentMemo(asset, owner, minOut), nil
}
