package thorchain

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	thorchain "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

// TxOutStorageV1 is going to manage all the outgoing tx
type TxOutStorageV1 struct {
	keeper        keeper.Keeper
	constAccessor constants.ConstantValues
	eventMgr      EventManager
	gasManager    GasManager
}

// NewTxOutStorage will create a new instance of TxOutStore.
func NewTxOutStorageV1(keeper keeper.Keeper,
	constAccessor constants.ConstantValues,
	eventMgr EventManager,
	gasManager GasManager) *TxOutStorageV1 {
	return &TxOutStorageV1{
		keeper:        keeper,
		eventMgr:      eventMgr,
		constAccessor: constAccessor,
		gasManager:    gasManager,
	}
}

func (tos *TxOutStorageV1) GetBlockOut(ctx cosmos.Context) (*TxOut, error) {
	return tos.keeper.GetTxOut(ctx, ctx.BlockHeight())
}

func (tos *TxOutStorageV1) GetOutboundItems(ctx cosmos.Context) ([]*TxOutItem, error) {
	block, err := tos.keeper.GetTxOut(ctx, ctx.BlockHeight())
	if block == nil {
		return nil, nil
	}
	return block.TxArray, err
}

func (tos *TxOutStorageV1) GetOutboundItemByToAddress(ctx cosmos.Context, to common.Address) []TxOutItem {
	filterItems := make([]TxOutItem, 0)
	items, _ := tos.GetOutboundItems(ctx)
	for _, item := range items {
		if item.ToAddress.Equals(to) {
			filterItems = append(filterItems, *item)
		}
	}
	return filterItems
}

func (tos *TxOutStorageV1) ClearOutboundItems(_ cosmos.Context) {} // do nothing

// TryAddTxOutItem add an outbound tx to block
// return bool indicate whether the transaction had been added successful or not
// return error indicate error
func (tos *TxOutStorageV1) TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi *TxOutItem) (bool, error) {
	success, err := tos.prepareTxOutItem(ctx, toi)
	if err != nil {
		return success, fmt.Errorf("fail to prepare outbound tx: %w", err)
	}
	if !success {
		return false, nil
	}
	// add tx to block out
	if err := tos.addToBlockOut(ctx, mgr, toi); err != nil {
		return false, err
	}
	return true, nil
}

// UnSafeAddTxOutItem - blindly adds a tx out, skipping vault selection, transaction
// fee deduction, etc
func (tos *TxOutStorageV1) UnSafeAddTxOutItem(ctx cosmos.Context, mgr Manager, toi *TxOutItem) error {
	return tos.addToBlockOut(ctx, mgr, toi)
}

// PrepareTxOutItem will do some data validation which include the following
// 1. Make sure it has a legitimate memo
// 2. choose an appropriate pool,Yggdrasil or Asgard
// 3. deduct transaction fee, keep in mind, only take transaction fee when active nodes are  more then minimumBFT
// return bool indicated whether the given TxOutItem should be added into block or not
func (tos *TxOutStorageV1) prepareTxOutItem(ctx cosmos.Context, toi *TxOutItem) (bool, error) {
	// Default the memo to the standard outbound memo
	if toi.Memo == "" {
		toi.Memo = NewOutboundMemo(toi.InHash).String()
	}

	if !toi.Chain.Equals(common.THORChain) {
		// If THORNode don't have a pool already selected to send from, discover one.
		if toi.VaultPubKey.IsEmpty() {
			// When deciding which Yggdrasil pool will send out our tx out, we
			// should consider which ones observed the inbound request tx, as
			// yggdrasil pools can go offline. Here THORNode get the voter record and
			// only consider Yggdrasils where their observed saw the "correct"
			// tx.

			activeNodeAccounts, err := tos.keeper.ListActiveNodeAccounts(ctx)
			if len(activeNodeAccounts) > 0 && err == nil {
				voter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
				if err != nil {
					return false, fmt.Errorf("fail to get observed tx voter: %w", err)
				}
				tx := voter.GetTx(activeNodeAccounts)

				// collect yggdrasil pools
				yggs, err := tos.collectYggdrasilPools(ctx, tx, toi.Chain.GetGasAsset())
				if err != nil {
					return false, fmt.Errorf("fail to collect yggdrasil pool: %w", err)
				}

				vault := yggs.SelectByMaxCoin(toi.Coin.Asset)
				// if none of the ygg vaults have enough funds, don't select one
				// and we'll select an asgard vault a few lines down
				if toi.Coin.Amount.LT(vault.GetCoin(toi.Coin.Asset).Amount) {
					toi.VaultPubKey = vault.PubKey
				}
			}
		}

		// Apparently we couldn't find a yggdrasil vault to send from, so use asgard
		if toi.VaultPubKey.IsEmpty() {

			active, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				ctx.Logger().Error("fail to get active vaults", "error", err)
			}

			vault := active.SelectByMaxCoin(toi.Coin.Asset)
			if vault.IsEmpty() {
				return false, fmt.Errorf("empty vault, cannot send out fund: %s", toi.Coin)
			}

			// check that this vault has enough funds to satisfy the request
			if toi.Coin.Amount.GT(vault.GetCoin(toi.Coin.Asset).Amount) {
				// not enough funds
				return false, fmt.Errorf("vault %s, does not have enough funds. Has %s, but requires %s", vault.PubKey, vault.GetCoin(toi.Coin.Asset), toi.Coin)
			}

			toi.VaultPubKey = vault.PubKey
		}

		// Ensure THORNode are not sending from and to the same address
		fromAddr, err := toi.VaultPubKey.GetAddress(toi.Chain)
		if err != nil || fromAddr.IsEmpty() || toi.ToAddress.Equals(fromAddr) {
			return false, err
		}
	}

	// Ensure the InHash is set
	if toi.InHash == "" {
		toi.InHash = common.BlankTxID
	}
	// calculate the MaxGas this tx can use
	if err := tos.calculateMaxGas(ctx, toi); err != nil {
		return false, fmt.Errorf("fail to calculate MaxGas:%w", err)
	}
	memo, err := ParseMemo(toi.Memo)
	if err != nil {
		return false, fmt.Errorf("fail to parse memo: %w", err)
	}
	if err := tos.deductTransactionFee(ctx, toi, memo); err != nil {
		return false, fmt.Errorf("fail to deduct transaction fee: %w", err)
	}
	// When we request Yggdrasil pool to return the fund, the coin field is actually empty
	// Signer when it sees an tx out item with memo "yggdrasil-" it will query the account on relevant chain
	// and coin field will be filled there, thus we have to let this one go
	if toi.Coin.IsEmpty() && !memo.IsType(TxYggdrasilReturn) {
		ctx.Logger().Info("tx out item has zero coin", toi.String())
		return false, nil
	}

	// increment out number of out tx for this in tx
	voter, err := tos.keeper.GetObservedTxInVoter(ctx, toi.InHash)
	if err != nil {
		return false, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	if voter.Height == 0 {
		voter.Height = ctx.BlockHeight()
	}
	voter.Actions = append(voter.Actions, *toi)
	tos.keeper.SetObservedTxInVoter(ctx, voter)

	return true, nil
}

func (tos *TxOutStorageV1) addToBlockOut(ctx cosmos.Context, mgr Manager, toi *TxOutItem) error {
	if toi.Chain.Equals(common.THORChain) {
		return tos.nativeTxOut(ctx, mgr, toi)
	}

	hash, err := toi.TxHash()
	if err != nil {
		return err
	}

	// add a tx marker
	mark := NewTxMarker(ctx.BlockHeight(), toi.Memo)
	memo, _ := ParseMemo(toi.Memo)
	if memo.IsInternal() {
		// need to add twice because observed inbound and outbound handler will observe
		err = tos.keeper.AppendTxMarker(ctx, hash, mark)
		if err != nil {
			return err
		}
	}
	err = tos.keeper.AppendTxMarker(ctx, hash, mark)
	if err != nil {
		return err
	}
	// since we're storing the memo in the tx market, we can clear it
	toi.Memo = ""

	return tos.keeper.AppendTxOut(ctx, ctx.BlockHeight(), toi)
}

// calculateMaxGas will calculate the Maximum gas coin bifrost can pay when sending out this transaction
func (tos *TxOutStorageV1) calculateMaxGas(ctx cosmos.Context, toi *TxOutItem) error {
	// this method is not going to calculate the fee again if the MaxGas field had been specified in the TxOutItem
	if !toi.MaxGas.IsEmpty() {
		return nil
	}
	transactionFee := tos.gasManager.GetFee(ctx, toi.Chain)
	if transactionFee.Asset.IsRune() {
		// given transaction fee is in RUNE , thus need to convert RUNE into equivalent asset
		p, err := tos.keeper.GetPool(ctx, toi.Coin.Asset)
		if err != nil {
			return fmt.Errorf("fail to get pool(%s):%w", toi.Coin.Asset, err)
		}
		toi.MaxGas = common.Gas{
			common.NewCoin(toi.Coin.Asset, p.RuneValueInAsset(transactionFee.Amount)),
		}
		return nil
	}
	toi.MaxGas = common.Gas{
		common.NewCoin(transactionFee.Asset, transactionFee.Amount.QuoUint64(2)),
	}
	return nil
}

// deductTransactionFee from TxOutItem
func (tos *TxOutStorageV1) deductTransactionFee(ctx cosmos.Context, toi *TxOutItem, memo thorchain.Memo) error {
	// Deduct TransactionFee from TOI and add to Reserve

	// for the following few transaction , it is THORNode internal transaction , thus doesn't charge fee
	if memo.IsType(TxYggdrasilFund) || memo.IsType(TxYggdrasilReturn) || memo.IsType(TxMigrate) || memo.IsType(TxRagnarok) {
		return nil
	}
	pool, err := tos.keeper.GetPool(ctx, toi.Chain.GetGasAsset())
	if err != nil {
		return fmt.Errorf("fail to get pool: %w", err)
	}

	transactionFee := tos.gasManager.GetFee(ctx, toi.Chain)
	var runeFee cosmos.Uint
	var assetFee cosmos.Uint
	if toi.Coin.Asset.IsRune() {
		// can't pay GAS in RUNE , thus need to convert it to asset
		if transactionFee.Asset.IsRune() {
			assetFee = pool.RuneValueInAsset(transactionFee.Amount)
			runeFee = transactionFee.Amount
		} else {
			assetFee = transactionFee.Amount
			runeFee = pool.AssetValueInRune(transactionFee.Amount)
		}
		if toi.Coin.Amount.LTE(runeFee) {
			runeFee = toi.Coin.Amount // Fee is the full amount
		}

		toi.Coin.Amount = common.SafeSub(toi.Coin.Amount, runeFee)
		fee := common.NewFee(common.Coins{common.NewCoin(toi.Coin.Asset, runeFee)}, assetFee)
		if err := tos.eventMgr.EmitFeeEvent(ctx, NewEventFee(toi.InHash, fee)); err != nil {
			ctx.Logger().Error("Failed to emit fee event", "error", err)
		}

		if err := tos.keeper.AddFeeToReserve(ctx, runeFee); err != nil {
			// Add to reserve
			ctx.Logger().Error("fail to add fee to reserve", "error", err)
		}

	} else {

		if transactionFee.Asset.IsRune() {
			runeFee = transactionFee.Amount
			assetFee = pool.RuneValueInAsset(transactionFee.Amount)
		} else {
			runeFee = pool.AssetValueInRune(transactionFee.Amount)
			assetFee = transactionFee.Amount
		}
		if toi.Coin.Amount.LTE(assetFee) {
			assetFee = toi.Coin.Amount // Fee is the full amount
			runeFee = pool.AssetValueInRune(assetFee)
		}

		toi.Coin.Amount = common.SafeSub(toi.Coin.Amount, assetFee) // Deduct Asset fee
		pool.BalanceAsset = pool.BalanceAsset.Add(assetFee)         // Add Asset fee to Pool
		var poolDeduct cosmos.Uint
		if runeFee.GT(pool.BalanceRune) {
			poolDeduct = pool.BalanceRune
		} else {
			poolDeduct = runeFee
		}
		pool.BalanceRune = common.SafeSub(pool.BalanceRune, runeFee) // Deduct Rune from Pool
		fee := common.NewFee(common.Coins{common.NewCoin(toi.Coin.Asset, assetFee)}, poolDeduct)
		if err := tos.eventMgr.EmitFeeEvent(ctx, NewEventFee(toi.InHash, fee)); err != nil {
			ctx.Logger().Error("Failed to emit fee event", "error", err)
		}
		if err := tos.keeper.SetPool(ctx, pool); err != nil { // Set Pool
			return fmt.Errorf("fail to save pool: %w", err)
		}
		if err := tos.keeper.AddFeeToReserve(ctx, runeFee); err != nil {
			return fmt.Errorf("fail to add fee to reserve: %w", err)
		}
	}
	return nil
}
func (tos *TxOutStorageV1) nativeTxOut(ctx cosmos.Context, mgr Manager, toi *TxOutItem) error {
	supplier := tos.keeper.Supply()

	addr, err := cosmos.AccAddressFromBech32(toi.ToAddress.String())
	if err != nil {
		return err
	}

	if toi.ModuleName == "" {
		toi.ModuleName = AsgardName
	}

	// send funds from module
	sdkErr := tos.keeper.SendFromModuleToAccount(ctx, toi.ModuleName, addr, toi.Coin)
	if sdkErr != nil {
		return errors.New(sdkErr.Error())
	}

	from, err := common.NewAddress(supplier.GetModuleAddress(AsgardName).String())
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return err
	}

	transactionFee := tos.constAccessor.GetInt64Value(constants.TransactionFee)

	tx := common.NewTx(
		common.BlankTxID,
		from,
		toi.ToAddress,
		common.Coins{toi.Coin},
		common.Gas{common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(transactionFee)))},
		toi.Memo,
	)

	active, err := tos.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "err", err)
		return err
	}

	observedTx := ObservedTx{
		ObservedPubKey: active[0].PubKey,
		BlockHeight:    ctx.BlockHeight(),
		Tx:             tx,
	}
	m, err := processOneTxIn(ctx, tos.keeper, observedTx, supplier.GetModuleAddress(AsgardName))
	if err != nil {
		ctx.Logger().Error("fail to process txOut", "error", err, "tx", tx.String())
		return err
	}

	handler := NewInternalHandler(tos.keeper, mgr)

	_, err = handler(ctx, m)
	if err != nil {
		ctx.Logger().Error("TxOut Handler failed:", "error", err)
		return err
	}

	return nil
}

func (tos *TxOutStorageV1) collectYggdrasilPools(ctx cosmos.Context, tx ObservedTx, gasAsset common.Asset) (Vaults, error) {
	// collect yggdrasil pools
	var vaults Vaults
	iterator := tos.keeper.GetVaultIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var vault Vault
		if err := tos.keeper.Cdc().UnmarshalBinaryBare(iterator.Value(), &vault); err != nil {
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if !vault.IsYggdrasil() {
			continue
		}
		// When trying to choose a ygg pool candidate to send out fund , let's make sure the ygg pool has gasAsset , for example, if it is
		// on Binance chain , make sure ygg pool has BNB asset in it , otherwise it won't be able to pay the transaction fee
		if !vault.HasAsset(gasAsset) {
			continue
		}

		// if THORNode are already sending assets from this ygg pool, deduct them.
		addr, err := vault.PubKey.GetThorAddress()
		if err != nil {
			return nil, fmt.Errorf("fail to get thor address from pub key(%s):%w", vault.PubKey, err)
		}

		// if the ygg pool didn't observe the TxIn, and didn't sign the TxIn, THORNode is not going to choose them to send out fund , because they might offline
		if !tx.HasSigned(addr) {
			continue
		}

		block, err := tos.GetBlockOut(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get block:%w", err)
		}

		// comments for future reference, this part of logic confuse me quite a few times
		// This method read the vault from key value store, and trying to find out all the ygg candidate that can be used to send out fund
		// given the fact, there might have multiple TxOutItem get created with in one block, and the fund has not been deducted from vault and save back to key values store,
		// thus every previously processed TxOut need to be deducted from the ygg vault to make sure THORNode has a correct view of the ygg funds

		for _, tx := range block.TxArray {
			if !tx.VaultPubKey.Equals(vault.PubKey) {
				continue
			}
			for i, yggCoin := range vault.Coins {
				if !yggCoin.Asset.Equals(tx.Coin.Asset) {
					continue
				}
				vault.Coins[i].Amount = common.SafeSub(vault.Coins[i].Amount, tx.Coin.Amount)
			}
		}

		vaults = append(vaults, vault)
	}

	return vaults, nil
}
