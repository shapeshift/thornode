package thorchain

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// SlasherV43 is v1 implementation os slasher
type SlasherV43 struct {
	keeper   keeper.Keeper
	eventMgr EventManager
}

// newSlasherV43 create a new instance of Slasher
func newSlasherV43(keeper keeper.Keeper, eventMgr EventManager) *SlasherV43 {
	return &SlasherV43{keeper: keeper, eventMgr: eventMgr}
}

// BeginBlock called when a new block get proposed to detect whether there are duplicate vote
func (s *SlasherV43) BeginBlock(ctx cosmos.Context, req abci.RequestBeginBlock, constAccessor constants.ConstantValues) {
	// Iterate through any newly discovered evidence of infraction
	// Slash any validators (and since-unbonded liquidity within the unbonding period)
	// who contributed to valid infractions
	for _, evidence := range req.ByzantineValidators {
		switch evidence.Type {
		case abci.EvidenceType_DUPLICATE_VOTE:
			if err := s.HandleDoubleSign(ctx, evidence.Validator.Address, evidence.Height, constAccessor); err != nil {
				ctx.Logger().Error("fail to slash for double signing a block", "error", err)
			}
		default:
			ctx.Logger().Error("ignored unknown evidence type", "type", evidence.Type)
		}
	}
}

// HandleDoubleSign - slashes a validator for singing two blocks at the same
// block height
// https://blog.cosmos.network/consensus-compare-casper-vs-tendermint-6df154ad56ae
func (s *SlasherV43) HandleDoubleSign(ctx cosmos.Context, addr crypto.Address, infractionHeight int64, constAccessor constants.ConstantValues) error {
	// check if we're recent enough to slash for this behavior
	maxAge := constAccessor.GetInt64Value(constants.DoubleSignMaxAge)
	if (common.BlockHeight(ctx) - infractionHeight) > maxAge {
		ctx.Logger().Info("double sign detected but too old to be slashed", "infraction height", fmt.Sprintf("%d", infractionHeight), "address", addr.String())
		return nil
	}

	nas, err := s.keeper.ListActiveValidators(ctx)
	if err != nil {
		return err
	}

	for _, na := range nas {
		pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, na.ValidatorConsPubKey)
		if err != nil {
			return err
		}

		if addr.String() == pk.Address().String() {
			if na.Bond.IsZero() {
				return fmt.Errorf("found account to slash for double signing, but did not have any bond to slash: %s", addr)
			}
			// take 5% of the minimum bond, and put it into the reserve
			minBond, err := s.keeper.GetMimir(ctx, constants.MinimumBondInRune.String())
			if minBond < 0 || err != nil {
				minBond = constAccessor.GetInt64Value(constants.MinimumBondInRune)
			}
			slashAmount := cosmos.NewUint(uint64(minBond)).MulUint64(5).QuoUint64(100)
			if slashAmount.GT(na.Bond) {
				slashAmount = na.Bond
			}
			na.Bond = common.SafeSub(na.Bond, slashAmount)
			coin := common.NewCoin(common.RuneNative, slashAmount)
			if err := s.keeper.SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
				ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
				return fmt.Errorf("fail to transfer funds from bond to reserve: %w", err)
			}

			return s.keeper.SetNodeAccount(ctx, na)
		}
	}

	return fmt.Errorf("could not find node account with validator address: %s", addr)
}

// LackObserving Slash node accounts that didn't observe a single inbound txn
func (s *SlasherV43) LackObserving(ctx cosmos.Context, constAccessor constants.ConstantValues) error {
	signingTransPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	height := common.BlockHeight(ctx)
	if height < signingTransPeriod {
		return nil
	}
	heightToCheck := height - signingTransPeriod
	tx, err := s.keeper.GetTxOut(ctx, heightToCheck)
	if err != nil {
		return fmt.Errorf("fail to get txout for block height(%d): %w", heightToCheck, err)
	}
	// no txout , return
	if tx == nil || tx.IsEmpty() {
		return nil
	}
	for _, item := range tx.TxArray {
		if item.InHash.IsEmpty() {
			continue
		}
		if item.InHash.Equals(common.BlankTxID) {
			continue
		}
		if err := s.slashNotObserving(ctx, item.InHash, constAccessor); err != nil {
			ctx.Logger().Error("fail to slash not observing", "error", err)
		}
	}

	return nil
}

func (s *SlasherV43) slashNotObserving(ctx cosmos.Context, txHash common.TxID, constAccessor constants.ConstantValues) error {
	voter, err := s.keeper.GetObservedTxInVoter(ctx, txHash)
	if err != nil {
		return fmt.Errorf("fail to get observe txin voter (%s): %w", txHash.String(), err)
	}

	if len(voter.Txs) == 0 {
		return nil
	}

	nodes, err := s.keeper.ListActiveValidators(ctx)
	if err != nil {
		return fmt.Errorf("unable to get list of active accounts: %w", err)
	}
	if len(voter.Txs) > 0 {
		tx := voter.Tx
		if !tx.IsEmpty() && len(tx.Signers) > 0 {
			height := voter.Height
			if tx.IsFinal() {
				height = voter.FinalisedHeight
			}
			s.checkSignerAndSlash(ctx, nodes, height, tx.GetSigners(), constAccessor)
		}
	}
	return nil
}

func (s *SlasherV43) checkSignerAndSlash(ctx cosmos.Context, nodes NodeAccounts, blockHeight int64, signers []cosmos.AccAddress, constAccessor constants.ConstantValues) {
	for _, na := range nodes {
		// the node is active after the tx finalised
		if na.ActiveBlockHeight > blockHeight {
			continue
		}
		found := false
		for _, addr := range signers {
			if na.NodeAddress.Equals(addr) {
				found = true
				break
			}
		}
		// this na is not found, therefore it should be slashed
		if !found {
			lackOfObservationPenalty := constAccessor.GetInt64Value(constants.LackOfObservationPenalty)
			if err := s.keeper.IncNodeAccountSlashPoints(ctx, na.NodeAddress, lackOfObservationPenalty); err != nil {
				ctx.Logger().Error("fail to inc slash points", "error", err)
			}
		}
	}
}

// LackSigning slash account that fail to sign tx
func (s *SlasherV43) LackSigning(ctx cosmos.Context, constAccessor constants.ConstantValues, mgr Manager) error {
	var resultErr error
	signingTransPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	if common.BlockHeight(ctx) < signingTransPeriod {
		return nil
	}
	height := common.BlockHeight(ctx) - signingTransPeriod
	txs, err := s.keeper.GetTxOut(ctx, height)
	if err != nil {
		return fmt.Errorf("fail to get txout from block height(%d): %w", height, err)
	}
	for i, tx := range txs.TxArray {
		if !tx.Chain.IsValidAddress(tx.ToAddress) {
			continue
		}
		if tx.OutHash.IsEmpty() {
			// Slash node account for not sending funds
			vault, err := s.keeper.GetVault(ctx, tx.VaultPubKey)
			if err != nil {
				// in some edge cases, when a txout item had been schedule to be send out by an yggdrasil vault
				// however the node operator decide to quit by sending a leave command, which will result in the vault get removed
				// if that happen , txout item should be scheduled to send out using asgard, thus when if fail to get vault , just
				// log the error, and continue
				ctx.Logger().Error("Unable to get vault", "error", err, "vault pub key", tx.VaultPubKey.String())
			}
			// slash if its a yggdrasil vault
			if vault.IsYggdrasil() {
				na, err := s.keeper.GetNodeAccountByPubKey(ctx, tx.VaultPubKey)
				if err != nil {
					ctx.Logger().Error("Unable to get node account", "error", err, "vault pub key", tx.VaultPubKey.String())
					continue
				}
				if err := s.keeper.IncNodeAccountSlashPoints(ctx, na.NodeAddress, signingTransPeriod*2); err != nil {
					ctx.Logger().Error("fail to inc slash points", "error", err, "node addr", na.NodeAddress.String())
				}
				if err := mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, signingTransPeriod*2, fmt.Sprintf("fail to sign out tx after %d blocks", signingTransPeriod))); err != nil {
					ctx.Logger().Error("fail to emit slash point event")
				}
				releaseHeight := common.BlockHeight(ctx) + (signingTransPeriod * 2)
				reason := "fail to send yggdrasil transaction"
				if err := s.keeper.SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
					ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
				}
			}

			voter, err := s.keeper.GetObservedTxInVoter(ctx, tx.InHash)
			if err != nil {
				ctx.Logger().Error("fail to get observed tx voter", "error", err)
				resultErr = fmt.Errorf("failed to get observed tx voter: %w", err)
				continue
			}

			active, err := s.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				return fmt.Errorf("fail to get active asgard vaults: %w", err)
			}
			available := active.Has(tx.Coin).SortBy(tx.Coin.Asset)
			if len(available) == 0 {
				// we need to give it somewhere to send from, even if that
				// asgard doesn't have enough funds. This is because if we
				// don't the transaction will just be dropped on the floor,
				// which is bad. Instead it may try to send from an asgard that
				// doesn't have enough funds, fail, and then get rescheduled
				// again later. Maybe by then the network will have enough
				// funds to satisfy.
				// TODO add split logic to send it out from multiple asgards in
				// this edge case.
				ctx.Logger().Error("unable to determine asgard vault to send funds, trying first asgard")
				if len(active) > 0 {
					vault = active[0]
				}
			} else {
				// each time we reschedule a transaction, we take the age of
				// the transaction, and move it to an vault that has less funds
				// than last time. This is here to ensure that if an asgard
				// vault becomes unavailable, the network will reschedule the
				// transaction on a different asgard vault.
				age := common.BlockHeight(ctx) - voter.FinalisedHeight
				if vault.IsYggdrasil() {
					// since the last attempt was a yggdrasil vault, lets
					// artificially inflate the age to ensure that the first
					// attempt is the largest asgard vault with funds
					age -= signingTransPeriod
					if age < 0 {
						age = 0
					}
				}
				rep := int(age / signingTransPeriod)
				if vault.PubKey.Equals(available[rep%len(available)].PubKey) {
					// looks like the new vault is going to be the same as the
					// old vault, increment rep to ensure a differ asgard is
					// chosen (if there is more than one option)
					rep++
				}
				vault = available[rep%len(available)]
			}

			// update original tx action in observed tx
			// check observedTx has done status. Skip if it does already.
			voterTx := voter.GetTx(NodeAccounts{})
			if voterTx.IsDone(len(voter.Actions)) {
				if len(voterTx.OutHashes) > 0 && len(voterTx.GetOutHashes()) > 0 {
					txs.TxArray[i].OutHash = voterTx.GetOutHashes()[0]
				}
				continue
			}

			// update the actions in the voter with the new vault pubkey
			for i, action := range voter.Actions {
				if action.Equals(tx) {
					voter.Actions[i].VaultPubKey = vault.PubKey
				}
			}
			s.keeper.SetObservedTxInVoter(ctx, voter)

			memo, _ := ParseMemo(mgr.GetVersion(), tx.Memo) // ignore err
			if memo.IsInternal() {
				// there is a different mechanism for rescheduling outbound
				// transactions for migration transactions
				continue
			}

			// Save the tx to as a new tx, select Asgard to send it this time.
			tx.VaultPubKey = vault.PubKey
			tx.GasRate = int64(mgr.GasMgr().GetGasRate(ctx, tx.Chain).Uint64())
			// if a pool with the asset name doesn't exist, skip rescheduling
			if !tx.Coin.Asset.IsRune() && !s.keeper.PoolExist(ctx, tx.Coin.Asset) {
				ctx.Logger().Error("fail to add outbound tx", "error", "coin is not rune and does not have an associated pool")
				continue
			}

			err = mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, tx)
			if err != nil {
				ctx.Logger().Error("fail to add outbound tx", "error", err)
				resultErr = fmt.Errorf("failed to add outbound tx: %w", err)
				continue
			}
		}
	}
	if !txs.IsEmpty() {
		if err := s.keeper.SetTxOut(ctx, txs); err != nil {
			return fmt.Errorf("fail to save tx out : %w", err)
		}
	}

	return resultErr
}

// SlashVault thorchain keep monitoring the outbound tx from asgard pool
// and yggdrasil pool, usually the txout is triggered by thorchain itself by
// adding an item into the txout array, refer to TxOutItem for the detail, the
// TxOutItem contains a specific coin and amount.  if somehow thorchain
// discover signer send out fund more than the amount specified in TxOutItem,
// it will slash the node account who does that by taking 1.5 * extra fund from
// node account's bond and subsidise the pool that actually lost it.
func (s *SlasherV43) SlashVault(ctx cosmos.Context, vaultPK common.PubKey, coins common.Coins, mgr Manager) error {
	if coins.IsEmpty() {
		return nil
	}
	vault, err := s.keeper.GetVault(ctx, vaultPK)
	if err != nil {
		return fmt.Errorf("fail to get slash vault (pubkey %s), %w", vaultPK, err)
	}
	membership := vault.GetMembership()

	// sum the total bond of membership of the vault
	totalBond := cosmos.ZeroUint()
	for _, member := range membership {
		na, err := s.keeper.GetNodeAccountByPubKey(ctx, member)
		if err != nil {
			ctx.Logger().Error("fail to get node account bond", "pk", member, "error", err)
			continue
		}
		totalBond = totalBond.Add(na.Bond)
	}

	for _, member := range membership {
		na, err := s.keeper.GetNodeAccountByPubKey(ctx, member)
		if err != nil {
			ctx.Logger().Error("fail to get node account for slash", "pk", member, "error", err)
			continue
		}

		for _, coin := range coins {
			if coin.IsEmpty() {
				continue
			}
			slashAmount := common.GetShare(na.Bond, totalBond, coin.Amount)
			ctx.Logger().Info("slash node account", "node address", na.NodeAddress.String(), "asset", coin.Asset.String(), "amount", slashAmount.String())

			// This check for rune actually isn't required, maybe should be
			// even removed entirely. Since a vault should never hold rune (as
			// its a native asset to THORChain and is NOT managed by threshold
			// signatures)
			if coin.Asset.IsRune() {
				// If rune, we take 1.5x the amount, and take it from their bond. We
				// put 1/3rd of it into the reserve, and 2/3rds into the pools (but
				// keeping the rune pool balances unchanged)
				amountToReserve := slashAmount.QuoUint64(2)
				// if the diff asset is RUNE , just took 1.5 * diff from their bond
				slashAmount = slashAmount.MulUint64(3).QuoUint64(2)
				if slashAmount.GT(na.Bond) {
					slashAmount = na.Bond
				}
				na.Bond = common.SafeSub(na.Bond, slashAmount)
				tx := common.Tx{}
				tx.ID = common.BlankTxID
				tx.FromAddress = na.BondAddress
				bondEvent := NewEventBond(slashAmount, BondCost, tx)
				if err := s.eventMgr.EmitEvent(ctx, bondEvent); err != nil {
					return fmt.Errorf("fail to emit bond event: %w", err)
				}
				totalBond = common.SafeSub(totalBond, slashAmount)
				if err := s.keeper.SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(common.NewCoin(common.RuneAsset(), amountToReserve))); err != nil {
					ctx.Logger().Error("fail to send slash funds to the reserve", "pk", member, "error", err)
					continue
				}

				continue
			}

			pool, err := s.keeper.GetPool(ctx, coin.Asset)
			if err != nil {
				ctx.Logger().Error("fail to get pool for slash", "asset", coin.Asset, "error", err)
				continue
			}
			// thorchain doesn't even have a pool for the asset
			if pool.IsEmpty() {
				ctx.Logger().Error("cannot slash for an empty pool", "asset", coin.Asset)
				continue
			}
			if slashAmount.GT(pool.BalanceAsset) {
				slashAmount = pool.BalanceAsset
			}
			runeValue := pool.AssetValueInRune(slashAmount).MulUint64(3).QuoUint64(2)
			if runeValue.GT(na.Bond) {
				runeValue = na.Bond
			}
			pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, slashAmount)
			pool.BalanceRune = pool.BalanceRune.Add(runeValue)
			na.Bond = common.SafeSub(na.Bond, runeValue)

			tx := common.Tx{}
			tx.ID = common.BlankTxID
			tx.FromAddress = na.BondAddress
			bondEvent := NewEventBond(runeValue, BondCost, tx)
			if err := s.eventMgr.EmitEvent(ctx, bondEvent); err != nil {
				return fmt.Errorf("fail to emit bond event: %w", err)
			}

			totalBond = common.SafeSub(totalBond, runeValue)
			if err := s.keeper.SetPool(ctx, pool); err != nil {
				ctx.Logger().Error("fail to save pool for slash", "asset", coin.Asset, "error", err)
				continue
			}

			poolSlashAmt := []PoolAmt{
				{
					Asset:  pool.Asset,
					Amount: 0 - int64(slashAmount.Uint64()),
				},
				{
					Asset:  common.RuneAsset(),
					Amount: int64(runeValue.Uint64()),
				},
			}
			eventSlash := NewEventSlash(pool.Asset, poolSlashAmt)
			if err := mgr.EventMgr().EmitEvent(ctx, eventSlash); err != nil {
				ctx.Logger().Error("fail to emit slash event", "error", err)
			}
		}

		// Ban the node account. Ensure we don't ban more than 1/3rd of any
		// given active or retiring vault
		if vault.IsYggdrasil() {
			toBan := true
			for _, vaultPk := range na.GetSignerMembership() {
				vault, err := s.keeper.GetVault(ctx, vaultPk)
				if err != nil {
					ctx.Logger().Error("fail to get vault", "error", err)
					continue
				}
				if !(vault.Status == ActiveVault || vault.Status == RetiringVault) {
					continue
				}
				activeMembers := 0
				for _, pk := range vault.GetMembership() {
					member, _ := s.keeper.GetNodeAccountByPubKey(ctx, pk)
					if member.Status == NodeActive {
						activeMembers += 1
					}
				}
				if !HasSuperMajority(activeMembers, len(vault.GetMembership())) {
					toBan = false
					break
				}
			}
			if toBan {
				na.ForcedToLeave = true
				na.LeaveScore = 1 // Set Leave Score to 1, which means the nodes is bad
			}
		}

		err = s.keeper.SetNodeAccount(ctx, na)
		if err != nil {
			ctx.Logger().Error("fail to save node account for slash", "error", err)
		}
	}
	return nil
}

// IncSlashPoints will increase the given account's slash points
func (s *SlasherV43) IncSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		if err := s.keeper.IncNodeAccountSlashPoints(ctx, addr, point); err != nil {
			ctx.Logger().Error("fail to increase node account slash point", "error", err, "address", addr.String())
		}
	}
}

// DecSlashPoints will decrease the given account's slash points
func (s *SlasherV43) DecSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		if err := s.keeper.DecNodeAccountSlashPoints(ctx, addr, point); err != nil {
			ctx.Logger().Error("fail to decrease node account slash point", "error", err, "address", addr.String())
		}
	}
}
