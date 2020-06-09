package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// GasMgrV1 implement GasManager interface which will store the gas related events happened in thorchain to memory
// emit GasEvent per block if there are any
type GasMgrV1 struct {
	gasEvent          *EventGas
	gas               common.Gas
	gasCount          map[common.Asset]int64
	constantsAccessor constants.ConstantValues
	keeper            keeper.Keeper
}

// NewGasMgrV1 create a new instance of GasMgrV1
func NewGasMgrV1(constantsAccessor constants.ConstantValues, k keeper.Keeper) *GasMgrV1 {
	return &GasMgrV1{
		gasEvent:          NewEventGas(),
		gas:               common.Gas{},
		gasCount:          make(map[common.Asset]int64, 0),
		constantsAccessor: constantsAccessor,
		keeper:            k,
	}
}

func (gm *GasMgrV1) reset() {
	gm.gasEvent = NewEventGas()
	gm.gas = common.Gas{}
	gm.gasCount = make(map[common.Asset]int64, 0)
}

// BeginBlock need to be called when a new block get created , update the internal EventGas to new one
func (gm *GasMgrV1) BeginBlock() {
	gm.reset()
}

// AddGasAsset to the EventGas
func (gm *GasMgrV1) AddGasAsset(gas common.Gas) {
	gm.gas = gm.gas.Add(gas)
	for _, coin := range gas {
		gm.gasCount[coin.Asset] += 1
	}
}

func (gm *GasMgrV1) GetGas() common.Gas {
	return gm.gas
}

// GetFee retrieve the network fee information from kv store, and calculate the dynamic fee customer should pay
// the return value is the amount of fee in RUNE
func (gm *GasMgrV1) GetFee(ctx cosmos.Context, chain common.Chain) int64 {
	transactionFee := gm.constantsAccessor.GetInt64Value(constants.TransactionFee)
	networkFee, err := gm.keeper.GetNetworkFee(ctx, chain)
	if err != nil {
		ctx.Logger().Error("fail to get network fee", "error", err)
		return transactionFee
	}
	if err := networkFee.Validate(); err != nil {
		ctx.Logger().Error("network fee is invalid", "error", err)
		return transactionFee
	}
	// Fee is calculated based on the network fee observed in previous block
	// THORNode is going to charge 3 times the fee it takes to send out the tx
	// 1.5 * fee will goes to vault
	// 1.5 * fee will become the max gas used to send out the tx
	pool, err := gm.keeper.GetPool(ctx, chain.GetGasAsset())
	if err != nil {
		ctx.Logger().Error("fail to get pool for %s: %w", chain.GetGasAsset(), err)
		return transactionFee
	}
	if pool.BalanceAsset.Equal(cosmos.ZeroUint()) || pool.BalanceRune.Equal(cosmos.ZeroUint()) {
		return transactionFee
	}

	return int64(pool.AssetValueInRune(cosmos.NewUint(networkFee.TransactionSize * networkFee.TransactionFeeRate * 3)).Uint64())
}

// GetMaxGas will calculate the maximum gas fee a tx can use
func (gm *GasMgrV1) GetMaxGas(ctx cosmos.Context, chain common.Chain) (common.Coin, error) {
	gasAsset := chain.GetGasAsset()

	pool, err := gm.keeper.GetPool(ctx, gasAsset)
	if err != nil {
		return common.NoCoin, fmt.Errorf("failed to get gas asset pool: %w", err)
	}
	transactionFee := gm.GetFee(ctx, chain)
	// max gas amount is the transaction fee divided by two, in asset amount
	maxAmt := pool.RuneValueInAsset(cosmos.NewUint(uint64(transactionFee / 2)))
	if chain.IsBNB() {
		// for Binance chain , the fee is fix, thus we give 1/3 as max gas
		maxAmt = pool.RuneValueInAsset(cosmos.NewUint(uint64(transactionFee / 3)))
	}
	return common.NewCoin(gasAsset, maxAmt), nil
}

// EndBlock emit the events
func (gm *GasMgrV1) EndBlock(ctx cosmos.Context, keeper keeper.Keeper, eventManager EventManager) {
	gm.ProcessGas(ctx, keeper)

	if len(gm.gasEvent.Pools) == 0 {
		return
	}

	if err := eventManager.EmitGasEvent(ctx, gm.gasEvent); nil != err {
		ctx.Logger().Error("fail to emit gas event", "error", err)
	}
	gm.reset()
}

// ProcessGas to subsidise the pool with RUNE for the gas they have spent
func (gm *GasMgrV1) ProcessGas(ctx cosmos.Context, keeper keeper.Keeper) {
	vault, err := keeper.GetVaultData(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get vault data", "error", err)
		return
	}
	for _, gas := range gm.gas {
		// if the coin is zero amount, don't need to do anything
		if gas.Amount.IsZero() {
			continue
		}

		pool, err := keeper.GetPool(ctx, gas.Asset)
		if err != nil {
			ctx.Logger().Error("fail to get pool", "pool", gas.Asset, "error", err)
			continue
		}
		if err := pool.Valid(); err != nil {
			ctx.Logger().Error("invalid pool", "pool", gas.Asset, "error", err)
			continue
		}
		runeGas := pool.AssetValueInRune(gas.Amount) // Convert to Rune (gas will never be RUNE)
		// If Rune owed now exceeds the Total Reserve, return it all
		if common.RuneAsset().Chain.Equals(common.THORChain) {
			if runeGas.LT(keeper.GetRuneBalaceOfModule(ctx, ReserveName)) {
				coin := common.NewCoin(common.RuneNative, runeGas)
				if err := keeper.SendFromModuleToModule(ctx, ReserveName, AsgardName, coin); err != nil {
					ctx.Logger().Error("fail to transfer funds from reserve to asgard", "pool", gas.Asset, "error", err)
					continue
				}
				pool.BalanceRune = pool.BalanceRune.Add(runeGas) // Add to the pool
			}
		} else {
			if runeGas.LTE(vault.TotalReserve) {
				vault.TotalReserve = common.SafeSub(vault.TotalReserve, runeGas) // Deduct from the Reserve.
				pool.BalanceRune = pool.BalanceRune.Add(runeGas)                 // Add to the pool
			} else {
				// since we didn't move any funds from reserve to the pool, set
				// the runeGas to zero so we emit the gas event to reflect the
				// appropriate amount
				runeGas = cosmos.ZeroUint()
			}
		}

		pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, gas.Amount)

		if err := keeper.SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to set pool", "pool", gas.Asset, "error", err)
			continue
		}

		gasPool := GasPool{
			Asset:    gas.Asset,
			AssetAmt: gas.Amount,
			RuneAmt:  runeGas,
			Count:    gm.gasCount[gas.Asset],
		}
		gm.gasEvent.UpsertGasPool(gasPool)
	}

	if err := keeper.SetVaultData(ctx, vault); err != nil {
		ctx.Logger().Error("fail to set vault data", "error", err)
	}
}
