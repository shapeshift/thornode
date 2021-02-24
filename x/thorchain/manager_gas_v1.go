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
func (gm *GasMgrV1) AddGasAsset(gas common.Gas, increaseTxCount bool) {
	gm.gas = gm.gas.Add(gas)
	if !increaseTxCount {
		return
	}
	for _, coin := range gas {
		gm.gasCount[coin.Asset]++
	}
}

// GetGas return gas
func (gm *GasMgrV1) GetGas() common.Gas {
	return gm.gas
}

// GetFee retrieve the network fee information from kv store, and calculate the dynamic fee customer should pay
// the return value is the amount of fee in RUNE
func (gm *GasMgrV1) GetFee(ctx cosmos.Context, chain common.Chain, asset common.Asset) cosmos.Uint {
	outboundTxFee, err := gm.keeper.GetMimir(ctx, constants.OutboundTransactionFee.String())
	if outboundTxFee < 0 || err != nil {
		outboundTxFee = gm.constantsAccessor.GetInt64Value(constants.OutboundTransactionFee)
	}
	transactionFee := cosmos.NewUint(uint64(outboundTxFee))
	if chain.Equals(common.THORChain) && asset.Chain.Equals(chain) {
		return transactionFee
	}
	networkFee, err := gm.keeper.GetNetworkFee(ctx, chain)
	if err != nil {
		ctx.Logger().Error("fail to get network fee", "error", err)
		return transactionFee
	}
	if err := networkFee.Valid(); err != nil {
		ctx.Logger().Error("network fee is invalid", "error", err, "chain", chain)
		return transactionFee
	}

	// Fee is calculated based on the network fee observed in previous block
	// THORNode is going to charge 3 times the fee it takes to send out the tx
	// 1.5 * fee will goes to vault
	// 1.5 * fee will become the max gas used to send out the tx
	fee := cosmos.NewUint(networkFee.TransactionSize * networkFee.TransactionFeeRate * 3)

	if asset.Equals(asset.Chain.GetGasAsset()) && chain.Equals(asset.Chain) {
		return fee
	}

	// convert gas asset value into rune
	pool, err := gm.keeper.GetPool(ctx, chain.GetGasAsset())
	if err != nil {
		ctx.Logger().Error("fail to get pool for %s: %w", chain.GetGasAsset(), err)
		return transactionFee
	}
	if pool.BalanceAsset.Equal(cosmos.ZeroUint()) || pool.BalanceRune.Equal(cosmos.ZeroUint()) {
		return transactionFee
	}

	fee = pool.AssetValueInRune(fee)
	if asset.IsRune() {
		return fee
	}

	// convert rune value into non-gas asset value
	pool, err = gm.keeper.GetPool(ctx, asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool for %s: %w", asset, err)
		return transactionFee
	}
	if pool.BalanceAsset.Equal(cosmos.ZeroUint()) || pool.BalanceRune.Equal(cosmos.ZeroUint()) {
		return transactionFee
	}
	return pool.RuneValueInAsset(fee)
}

// GetGasRate return the gas rate
func (gm *GasMgrV1) GetGasRate(ctx cosmos.Context, chain common.Chain) cosmos.Uint {
	outboundTxFee, err := gm.keeper.GetMimir(ctx, constants.OutboundTransactionFee.String())
	if outboundTxFee < 0 || err != nil {
		outboundTxFee = gm.constantsAccessor.GetInt64Value(constants.OutboundTransactionFee)
	}
	transactionFee := cosmos.NewUint(uint64(outboundTxFee))
	if chain.Equals(common.THORChain) {
		return transactionFee
	}
	networkFee, err := gm.keeper.GetNetworkFee(ctx, chain)
	if err != nil {
		ctx.Logger().Error("fail to get network fee", "error", err)
		return transactionFee
	}
	if err := networkFee.Valid(); err != nil {
		ctx.Logger().Error("network fee is invalid", "error", err, "chain", chain)
		return transactionFee
	}
	return cosmos.NewUint(networkFee.TransactionFeeRate * 3 / 2)
}

// GetMaxGas will calculate the maximum gas fee a tx can use
func (gm *GasMgrV1) GetMaxGas(ctx cosmos.Context, chain common.Chain) (common.Coin, error) {
	gasAsset := chain.GetGasAsset()
	var amount cosmos.Uint

	nf, err := gm.keeper.GetNetworkFee(ctx, chain)
	if err != nil {
		return common.NoCoin, fmt.Errorf("fail to get network fee for chain(%s): %w", chain, err)
	}
	if chain.IsBNB() {
		amount = cosmos.NewUint(nf.TransactionSize * nf.TransactionFeeRate)
	} else {
		amount = cosmos.NewUint(nf.TransactionSize * nf.TransactionFeeRate).MulUint64(3).QuoUint64(2)
	}

	return common.NewCoin(gasAsset, amount), nil
}

// SubGas will subtract the gas from the gas manager
func (gm *GasMgrV1) SubGas(gas common.Gas) {
	gm.gas = gm.gas.Sub(gas)
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
	gm.reset() // do not remove, will cause consensus failures
}

// ProcessGas to subsidise the pool with RUNE for the gas they have spent
func (gm *GasMgrV1) ProcessGas(ctx cosmos.Context, keeper keeper.Keeper) {
	if keeper.RagnarokInProgress(ctx) {
		// ragnarok is in progress , stop
		return
	}
	vault, err := keeper.GetNetwork(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get network data", "error", err)
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
		if runeGas.IsZero() {
			continue
		}
		// If Rune owed now exceeds the Total Reserve, return it all
		if runeGas.LT(keeper.GetRuneBalanceOfModule(ctx, ReserveName)) {
			coin := common.NewCoin(common.RuneNative, runeGas)
			if err := keeper.SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(coin)); err != nil {
				ctx.Logger().Error("fail to transfer funds from reserve to asgard", "pool", gas.Asset, "error", err)
				continue
			}
			pool.BalanceRune = pool.BalanceRune.Add(runeGas) // Add to the pool
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

	if err := keeper.SetNetwork(ctx, vault); err != nil {
		ctx.Logger().Error("fail to set network data", "error", err)
	}
}
