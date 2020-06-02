package thorchain

import (
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

// GetFee retrieve the network fee information from kv store, and calculate the fee customer should pay
// return fee is in the amount of gas asset for the given chain
// BTC , the return fee should be sats
// ETH , the return fee should be ETH
// Binance , the return fee should be in BNB
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
	return networkFee.TransactionSize * int64(networkFee.TransactionFeeRate.Uint64()) * 3
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
			if runeGas.LT(vault.TotalReserve) {
				vault.TotalReserve = common.SafeSub(vault.TotalReserve, runeGas) // Deduct from the Reserve.
				pool.BalanceRune = pool.BalanceRune.Add(runeGas)                 // Add to the pool
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
