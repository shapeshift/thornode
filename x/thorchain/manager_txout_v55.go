package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// TxOutStorageV55 is going to manage all the outgoing tx
type TxOutStorageV55 struct {
	keeper        keeper.Keeper
	constAccessor constants.ConstantValues
	eventMgr      EventManager
	gasManager    GasManager
}

// newTxOutStorageV55 will create a new instance of TxOutStore.
func newTxOutStorageV55(keeper keeper.Keeper, constAccessor constants.ConstantValues, eventMgr EventManager, gasManager GasManager) *TxOutStorageV55 {
	return &TxOutStorageV55{
		keeper:        keeper,
		eventMgr:      eventMgr,
		constAccessor: constAccessor,
		gasManager:    gasManager,
	}
}

func (tos *TxOutStorageV55) EndBlock(ctx cosmos.Context, mgr Manager) error { return nil }

// GetBlockOut read the TxOut from kv store
func (tos *TxOutStorageV55) GetBlockOut(ctx cosmos.Context) (*TxOut, error) {
	return tos.keeper.GetTxOut(ctx, common.BlockHeight(ctx))
}

// GetOutboundItems read all the outbound item from kv store
func (tos *TxOutStorageV55) GetOutboundItems(ctx cosmos.Context) ([]TxOutItem, error) {
	block, err := tos.keeper.GetTxOut(ctx, common.BlockHeight(ctx))
	if block == nil {
		return nil, nil
	}
	return block.TxArray, err
}

// GetOutboundItemByToAddress read all the outbound items filter by the given to address
func (tos *TxOutStorageV55) GetOutboundItemByToAddress(ctx cosmos.Context, to common.Address) []TxOutItem {
	filterItems := make([]TxOutItem, 0)
	items, _ := tos.GetOutboundItems(ctx)
	for _, item := range items {
		if item.ToAddress.Equals(to) {
			filterItems = append(filterItems, item)
		}
	}
	return filterItems
}

// ClearOutboundItems remove all the tx out items , mostly used for test
func (tos *TxOutStorageV55) ClearOutboundItems(ctx cosmos.Context) {
	_ = tos.keeper.ClearTxOut(ctx, common.BlockHeight(ctx))
}

// TryAddTxOutItem add an outbound tx to block
// return bool indicate whether the transaction had been added successful or not
// return error indicate error
func (tos *TxOutStorageV55) TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem) (bool, error) {
	outputs, err := tos.prepareTxOutItem(ctx, toi)
	if err != nil {
		return false, fmt.Errorf("fail to prepare outbound tx: %w", err)
	}
	if len(outputs) == 0 {
		return false, ErrNotEnoughToPayFee
	}
	// add tx to block out
	for _, output := range outputs {
		if err := tos.addToBlockOut(ctx, mgr, output); err != nil {
			return false, err
		}
	}
	return true, nil
}

// UnSafeAddTxOutItem - blindly adds a tx out, skipping vault selection, transaction
// fee deduction, etc
func (tos *TxOutStorageV55) UnSafeAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem) error {
	// BCH chain will convert legacy address to new format automatically , thus when observe it back can't be associated with the original inbound
	// so here convert the legacy address to new format
	if toi.Chain.Equals(common.BCHChain) {
		newBCHAddress, err := common.ConvertToNewBCHAddressFormat(toi.ToAddress)
		if err != nil {
			return fmt.Errorf("fail to convert BCH address to new format: %w", err)
		}
		if newBCHAddress.IsEmpty() {
			return fmt.Errorf("empty to address , can't send out")
		}
		toi.ToAddress = newBCHAddress
	}
	return tos.addToBlockOut(ctx, mgr, toi)
}

// prepareTxOutItem will do some data validation which include the following
// 1. Make sure it has a legitimate memo
// 2. choose an appropriate vault(s) to send from (ygg first, active asgard, then retiring asgard)
// 3. deduct transaction fee, keep in mind, only take transaction fee when active nodes are  more then minimumBFT
// return list of outbound transactions
func (tos *TxOutStorageV55) prepareTxOutItem(ctx cosmos.Context, toi TxOutItem) ([]TxOutItem, error) {
	var outputs []TxOutItem
	// Default the memo to the standard outbound memo
	if toi.Memo == "" {
		toi.Memo = NewOutboundMemo(toi.InHash).String()
	}
	// Ensure the InHash is set
	if toi.InHash.IsEmpty() {
		toi.InHash = common.BlankTxID
	}
	if toi.ToAddress.IsEmpty() {
		return outputs, fmt.Errorf("empty to address, can't send out")
	}
	if !toi.ToAddress.IsChain(toi.Chain) {
		return outputs, fmt.Errorf("to address(%s), is not of chain(%s)", toi.ToAddress, toi.Chain)
	}

	// BCH chain will convert legacy address to new format automatically , thus when observe it back can't be associated with the original inbound
	// so here convert the legacy address to new format
	if toi.Chain.Equals(common.BCHChain) {
		newBCHAddress, err := common.ConvertToNewBCHAddressFormat(toi.ToAddress)
		if err != nil {
			return outputs, fmt.Errorf("fail to convert BCH address to new format: %w", err)
		}
		if newBCHAddress.IsEmpty() {
			return outputs, fmt.Errorf("empty to address , can't send out")
		}
		toi.ToAddress = newBCHAddress
	}
	signingTransactionPeriod := tos.constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	transactionFeeRune := tos.gasManager.GetFee(ctx, toi.Chain, common.RuneAsset())
	transactionFeeAsset := tos.gasManager.GetFee(ctx, toi.Chain, toi.Coin.Asset)

	if toi.Chain.Equals(common.THORChain) {
		outputs = append(outputs, toi)
	} else {
		if !toi.VaultPubKey.IsEmpty() {
			// a vault is already manually selected, blindly go forth with that
			outputs = append(outputs, toi)
		} else {
			maxGasAsset, err := tos.gasManager.GetMaxGas(ctx, toi.Chain)
			if err != nil {
				ctx.Logger().Error("fail to get max gas asset", "error", err)
			}
			// THORNode don't have a vault already selected to send from, discover one.
			vaults := make(Vaults, 0) // a sorted list of vaults to send funds from

			// ///////////// COLLECT YGGDRASIL VAULTS ///////////////////////////
			// When deciding which Yggdrasil pool will send out our tx out, we
			// should consider which ones observed the inbound request tx, as
			// yggdrasil pools can go offline. Here THORNode get the voter record and
			// only consider Yggdrasils where their observed saw the "correct"
			// tx.

			activeNodeAccounts, err := tos.keeper.ListActiveValidators(ctx)
			if err != nil {
				ctx.Logger().Error("fail to get all active node accounts", "error", err)
			}
			if len(activeNodeAccounts) > 0 {
				voter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
				if err != nil {
					return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
				}
				tx := voter.GetTx(activeNodeAccounts)

				// collect yggdrasil pools is going to get a list of yggdrasil
				// vault that THORChain can used to send out fund
				yggs, err := tos.collectYggdrasilPools(ctx, tx, toi.Chain.GetGasAsset())
				if err != nil {
					return nil, fmt.Errorf("fail to collect yggdrasil pool: %w", err)
				}

				// add yggdrasil vaults first
				vaults = append(vaults, yggs.SortBy(toi.Coin.Asset)...)
			}
			// //////////////////////////////////////////////////////////////

			// ///////////// COLLECT ACTIVE ASGARD VAULTS ///////////////////
			active, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				ctx.Logger().Error("fail to get active vaults", "error", err)
			}

			for i := range active {
				active[i], err = tos.deductVaultPendingOutboundBalance(ctx, active[i])
				if err != nil {
					ctx.Logger().Error("fail to deduct outstanding outbound balance from asgard vault", "error", err)
					continue
				}
			}
			vaults = append(vaults, tos.keeper.SortBySecurity(ctx, active, signingTransactionPeriod)...)
			// //////////////////////////////////////////////////////////////

			// ///////////// COLLECT RETIRING ASGARD VAULTS /////////////////
			retiring, err := tos.keeper.GetAsgardVaultsByStatus(ctx, RetiringVault)
			if err != nil {
				ctx.Logger().Error("fail to get retiring vaults", "error", err)
			}
			for i := range retiring {
				retiring[i], err = tos.deductVaultPendingOutboundBalance(ctx, retiring[i])
				if err != nil {
					ctx.Logger().Error("fail to deduct outstanding outbound balance from asgard vault", "error", err)
					continue
				}
			}
			vaults = append(vaults, tos.keeper.SortBySecurity(ctx, retiring, signingTransactionPeriod)...)
			// //////////////////////////////////////////////////////////////

			// iterate over discovered vaults and find vaults to send funds from
			for _, vault := range vaults {
				// Ensure THORNode are not sending from and to the same address
				fromAddr, err := vault.PubKey.GetAddress(toi.Chain)
				if err != nil || fromAddr.IsEmpty() || toi.ToAddress.Equals(fromAddr) {
					continue
				}
				// if the asset in the vault is not enough to pay for the fee , then skip it
				if vault.GetCoin(toi.Coin.Asset).Amount.LTE(transactionFeeAsset) {
					continue
				}
				// if the vault doesn't have gas asset in it , or it doesn't have enough to pay for gas
				gasAsset := vault.GetCoin(toi.Chain.GetGasAsset())
				if gasAsset.IsEmpty() || gasAsset.Amount.LT(maxGasAsset.Amount) {
					continue
				}

				toi.VaultPubKey = vault.PubKey
				if toi.Coin.Amount.LTE(vault.GetCoin(toi.Coin.Asset).Amount) {
					outputs = append(outputs, toi)
					toi.Coin.Amount = cosmos.ZeroUint()
					break
				} else {
					toi.VaultPubKey = vault.PubKey
					remainingAmount := common.SafeSub(toi.Coin.Amount, vault.GetCoin(toi.Coin.Asset).Amount)
					toi.Coin.Amount = common.SafeSub(toi.Coin.Amount, remainingAmount)
					outputs = append(outputs, toi)
					toi.Coin.Amount = remainingAmount
				}
			}

			// Check we found enough funds to satisfy the request, error if we didn't
			if !toi.Coin.Amount.IsZero() {
				return nil, fmt.Errorf("insufficient funds for outbound request: %s %s remaining", toi.ToAddress.String(), toi.Coin.String())
			}
		}
	}
	var finalOutput []TxOutItem
	var pool Pool
	var feeEvents []*EventFee
	finalRuneFee := cosmos.ZeroUint()
	for i := range outputs {
		if outputs[i].MaxGas.IsEmpty() {
			maxGasCoin, err := tos.gasManager.GetMaxGas(ctx, outputs[i].Chain)
			if err != nil {
				return nil, fmt.Errorf("fail to get max gas coin: %w", err)
			}
			outputs[i].MaxGas = common.Gas{
				maxGasCoin,
			}
			// THOR Chain doesn't need to have max gas
			if outputs[i].MaxGas.IsEmpty() && !outputs[i].Chain.Equals(common.THORChain) {
				return nil, fmt.Errorf("max gas cannot be empty: %s", outputs[i].MaxGas)
			}
			outputs[i].GasRate = int64(tos.gasManager.GetGasRate(ctx, outputs[i].Chain).Uint64())
		}

		runeFee := transactionFeeRune // Fee is the prescribed fee

		// Deduct OutboundTransactionFee from TOI and add to Reserve
		memo, err := ParseMemo(tos.keeper.Version(), outputs[i].Memo) // ignore err
		if err == nil && !memo.IsType(TxYggdrasilFund) && !memo.IsType(TxYggdrasilReturn) && !memo.IsType(TxMigrate) && !memo.IsType(TxRagnarok) {
			if outputs[i].Coin.Asset.IsRune() {
				if outputs[i].Coin.Amount.LTE(transactionFeeRune) {
					runeFee = outputs[i].Coin.Amount // Fee is the full amount
				}
				finalRuneFee = finalRuneFee.Add(runeFee)
				outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, runeFee)
				fee := common.NewFee(common.Coins{common.NewCoin(outputs[i].Coin.Asset, runeFee)}, cosmos.ZeroUint())
				feeEvents = append(feeEvents, NewEventFee(outputs[i].InHash, fee, cosmos.ZeroUint()))
			} else {
				if pool.IsEmpty() {
					var err error
					pool, err = tos.keeper.GetPool(ctx, toi.Coin.Asset) // Get pool
					if err != nil {
						// the error is already logged within kvstore
						return nil, fmt.Errorf("fail to get pool: %w", err)
					}
				}

				// if pool units is zero, no asset fee is taken
				if !pool.GetPoolUnits().IsZero() {
					assetFee := transactionFeeAsset
					if outputs[i].Coin.Amount.LTE(assetFee) {
						assetFee = outputs[i].Coin.Amount // Fee is the full amount
					}

					outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, assetFee) // Deduct Asset fee
					if outputs[i].Coin.Asset.IsSyntheticAsset() {
						// burn the synth asset which used to pay for fee, that's only required when the synth is sending from asgard
						if outputs[i].ModuleName == "" || outputs[i].ModuleName == AsgardName {
							if err := tos.keeper.SendFromModuleToModule(ctx,
								AsgardName,
								ModuleName,
								common.NewCoins(common.NewCoin(outputs[i].Coin.Asset, assetFee))); err != nil {
								ctx.Logger().Error("fail to move synth asset fee from asgard to Module", "error", err)
							} else {
								if err := tos.keeper.BurnFromModule(ctx, ModuleName, common.NewCoin(outputs[i].Coin.Asset, assetFee)); err != nil {
									ctx.Logger().Error("fail to burn synth asset", "error", err)
								}
							}
						}
					} else {
						pool.BalanceAsset = pool.BalanceAsset.Add(assetFee) // Add Asset fee to Pool
					}
					var poolDeduct cosmos.Uint
					if runeFee.GT(pool.BalanceRune) {
						poolDeduct = pool.BalanceRune
					} else {
						poolDeduct = runeFee
					}
					finalRuneFee = finalRuneFee.Add(runeFee)
					pool.BalanceRune = common.SafeSub(pool.BalanceRune, runeFee) // Deduct Rune from Pool
					fee := common.NewFee(common.Coins{common.NewCoin(outputs[i].Coin.Asset, assetFee)}, poolDeduct)
					feeEvents = append(feeEvents, NewEventFee(outputs[i].InHash, fee, cosmos.ZeroUint()))
				}
			}
		}

		// when it is ragnarok , the network doesn't charge fee , however if the output asset is gas asset,
		// then the amount of max gas need to be taken away from the customer , otherwise the vault will be insolvent and doesn't
		// have enough to fulfill outbound
		// Also the MaxGas has not put back to pool ,so there is no need to subside pool when ragnarok is in progress
		if memo.IsType(TxRagnarok) && outputs[i].Coin.Asset.IsGasAsset() {
			gasAmt := outputs[i].MaxGas.ToCoins().GetCoin(outputs[i].Coin.Asset).Amount
			outputs[i].Coin.Amount = common.SafeSub(outputs[i].Coin.Amount, gasAmt)
		}
		// When we request Yggdrasil pool to return the fund, the coin field is actually empty
		// Signer when it sees an tx out item with memo "yggdrasil-" it will query the account on relevant chain
		// and coin field will be filled there, thus we have to let this one go

		if outputs[i].Coin.IsEmpty() && !memo.IsType(TxYggdrasilReturn) {
			ctx.Logger().Info("tx out item has zero coin", "tx_out", outputs[i].String())
			return nil, errors.New("tx out item has zero coin")
		}

		if !outputs[i].InHash.Equals(common.BlankTxID) {
			// increment out number of out tx for this in tx
			voter, err := tos.keeper.GetObservedTxInVoter(ctx, outputs[i].InHash)
			if err != nil {
				return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
			}
			voter.FinalisedHeight = common.BlockHeight(ctx)
			voter.Actions = append(voter.Actions, outputs[i])
			tos.keeper.SetObservedTxInVoter(ctx, voter)
		}

		finalOutput = append(finalOutput, outputs[i])
	}

	if !pool.IsEmpty() {
		if err := tos.keeper.SetPool(ctx, pool); err != nil { // Set Pool
			return nil, fmt.Errorf("fail to save pool: %w", err)
		}
	}
	for _, feeEvent := range feeEvents {
		if err := tos.eventMgr.EmitFeeEvent(ctx, feeEvent); err != nil {
			ctx.Logger().Error("fail to emit fee event", "error", err)
		}
	}
	if !finalRuneFee.IsZero() {
		if err := tos.keeper.AddFeeToReserve(ctx, finalRuneFee); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}
	}

	return finalOutput, nil
}

func (tos *TxOutStorageV55) addToBlockOut(ctx cosmos.Context, mgr Manager, toi TxOutItem) error {
	// THORChain , native RUNE will not need to forward the txout to bifrost
	if toi.Chain.Equals(common.THORChain) {
		return tos.nativeTxOut(ctx, mgr, toi)
	}

	return tos.keeper.AppendTxOut(ctx, common.BlockHeight(ctx), toi)
}

func (tos *TxOutStorageV55) nativeTxOut(ctx cosmos.Context, mgr Manager, toi TxOutItem) error {
	addr, err := cosmos.AccAddressFromBech32(toi.ToAddress.String())
	if err != nil {
		return err
	}

	if toi.ModuleName == "" {
		toi.ModuleName = AsgardName
	}

	// mint if we're sending from THORChain module
	if toi.ModuleName == ModuleName {
		if err := tos.keeper.MintToModule(ctx, toi.ModuleName, toi.Coin); err != nil {
			return fmt.Errorf("fail to mint coins during txout: %w", err)
		}
	}

	// send funds from module
	sdkErr := tos.keeper.SendFromModuleToAccount(ctx, toi.ModuleName, addr, common.NewCoins(toi.Coin))
	if sdkErr != nil {
		return errors.New(sdkErr.Error())
	}

	from, err := tos.keeper.GetModuleAddress(toi.ModuleName)
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return err
	}
	outboundTxFee, err := tos.keeper.GetMimir(ctx, constants.OutboundTransactionFee.String())
	if outboundTxFee < 0 || err != nil {
		outboundTxFee = tos.constAccessor.GetInt64Value(constants.OutboundTransactionFee)
	}

	tx := common.NewTx(
		common.BlankTxID,
		from,
		toi.ToAddress,
		common.Coins{toi.Coin},
		common.Gas{common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(outboundTxFee)))},
		toi.Memo,
	)

	active, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "err", err)
		return err
	}

	observedTx := ObservedTx{
		ObservedPubKey: active[0].PubKey,
		BlockHeight:    common.BlockHeight(ctx),
		Tx:             tx,
		FinaliseHeight: common.BlockHeight(ctx),
	}
	m, err := processOneTxInV46(ctx, tos.keeper, observedTx, tos.keeper.GetModuleAccAddress(AsgardName))
	if err != nil {
		ctx.Logger().Error("fail to process txOut", "error", err, "tx", tx.String())
		return err
	}

	handler := NewInternalHandler(mgr)

	_, err = handler(ctx, m)
	if err != nil {
		ctx.Logger().Error("TxOut Handler failed:", "error", err)
		return err
	}

	return nil
}

// collectYggdrasilPools is to get all the yggdrasil vaults , that THORChain can used to send out fund
func (tos *TxOutStorageV55) collectYggdrasilPools(ctx cosmos.Context, tx ObservedTx, gasAsset common.Asset) (Vaults, error) {
	// collect yggdrasil pools
	var vaults Vaults
	iterator := tos.keeper.GetVaultIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var vault Vault
		if err := tos.keeper.Cdc().Unmarshal(iterator.Value(), &vault); err != nil {
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if !vault.IsYggdrasil() {
			continue
		}
		// When trying to choose a ygg pool candidate to send out fund , let's
		// make sure the ygg pool has gasAsset , for example, if it is
		// on Binance chain , make sure ygg pool has BNB asset in it ,
		// otherwise it won't be able to pay the transaction fee
		if !vault.HasAsset(gasAsset) {
			continue
		}

		// if THORNode are already sending assets from this ygg pool, deduct them.
		addr, err := vault.PubKey.GetThorAddress()
		if err != nil {
			return nil, fmt.Errorf("fail to get thor address from pub key(%s):%w", vault.PubKey, err)
		}

		// if the ygg pool didn't observe the TxIn, and didn't sign the TxIn,
		// THORNode is not going to choose them to send out fund , because they
		// might offline
		if !tx.HasSigned(addr) {
			continue
		}

		jail, err := tos.keeper.GetNodeAccountJail(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("fail to get ygg jail:%w", err)
		}
		if jail.IsJailed(ctx) {
			continue
		}

		v, err := tos.deductVaultPendingOutboundBalance(ctx, vault)
		if err != nil {
			ctx.Logger().Error("fail to deduct vault outstanding outbound balance", "error", err)
			continue
		}
		vaults = append(vaults, v)
	}

	return vaults, nil
}

func (tos *TxOutStorageV55) deductVaultPendingOutboundBalance(ctx cosmos.Context, vault Vault) (Vault, error) {
	block, err := tos.GetBlockOut(ctx)
	if err != nil {
		return types.Vault{}, fmt.Errorf("fail to get block:%w", err)
	}

	// comments for future reference, this part of logic confuse me quite a few times
	// This method read the vault from key value store, and trying to find out all the ygg candidate that can be used to send out fund
	// given the fact, there might have multiple TxOutItem get created with in one block, and the fund has not been deducted from vault and save back to key values store,
	// thus every previously processed TxOut need to be deducted from the ygg vault to make sure THORNode has a correct view of the ygg funds
	vault = tos.deductVaultBlockPendingOutbound(vault, block)

	// go back SigningTransactionPeriod blocks to see whether there are outstanding tx, the vault need to send out
	// if there is , deduct it from their balance
	signingPeriod := tos.constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := block.Height - signingPeriod
	if startHeight < 1 {
		startHeight = 1
	}
	for i := startHeight; i < block.Height; i++ {
		blockOut, err := tos.keeper.GetTxOut(ctx, i)
		if err != nil {
			ctx.Logger().Error("fail to get block tx out", "error", err)
		}
		vault = tos.deductVaultBlockPendingOutbound(vault, blockOut)
	}
	return vault, nil
}

func (tos *TxOutStorageV55) deductVaultBlockPendingOutbound(vault Vault, block *TxOut) Vault {
	for _, txOutItem := range block.TxArray {
		if !txOutItem.VaultPubKey.Equals(vault.PubKey) {
			continue
		}
		// only still outstanding txout will be considered
		if !txOutItem.OutHash.IsEmpty() {
			continue
		}
		// deduct the gas asset from the vault as well
		var gasCoin common.Coin
		if !txOutItem.MaxGas.IsEmpty() {
			gasCoin = txOutItem.MaxGas.ToCoins().GetCoin(txOutItem.Chain.GetGasAsset())
		}
		for i, yggCoin := range vault.Coins {
			if yggCoin.Asset.Equals(txOutItem.Coin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, txOutItem.Coin.Amount)
			}
			if yggCoin.Asset.Equals(gasCoin.Asset) {
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, gasCoin.Amount)
			}
		}
	}
	return vault
}
