package thorchain

import (
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type SlasherV1 struct {
	keeper keeper.Keeper
}

// NewSlasher create a new instance of Slasher
func NewSlasherV1(keeper keeper.Keeper) *SlasherV1 {
	return &SlasherV1{
		keeper: keeper,
	}
}

func (s *SlasherV1) BeginBlock(ctx cosmos.Context, req abci.RequestBeginBlock, constAccessor constants.ConstantValues) {
	// Iterate through any newly discovered evidence of infraction
	// Slash any validators (and since-unbonded stake within the unbonding period)
	// who contributed to valid infractions
	for _, evidence := range req.ByzantineValidators {
		switch evidence.Type {
		case tmtypes.ABCIEvidenceTypeDuplicateVote:
			if err := s.HandleDoubleSign(ctx, evidence.Validator.Address, evidence.Height, constAccessor); err != nil {
				ctx.Logger().Error("fail to slash for double signing a block", "error", err)
			}
		default:
			ctx.Logger().Error(fmt.Sprintf("ignored unknown evidence type: %s", evidence.Type))
		}
	}
}

// HandleDoubleSign - slashes a validator for singing two blocks at the same
// block height
// https://blog.cosmos.network/consensus-compare-casper-vs-tendermint-6df154ad56ae
func (s *SlasherV1) HandleDoubleSign(ctx cosmos.Context, addr crypto.Address, infractionHeight int64, constAccessor constants.ConstantValues) error {
	// check if we're recent enough to slash for this behavior
	maxAge := constAccessor.GetInt64Value(constants.DoubleSignMaxAge)
	if (common.BlockHeight(ctx) - infractionHeight) > maxAge {
		ctx.Logger().Info("double sign detected but too old to be slashed", "infraction height", fmt.Sprintf("%d", infractionHeight), "address", addr.String())
		return nil
	}

	nas, err := s.keeper.ListActiveNodeAccounts(ctx)
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
			na.Bond = common.SafeSub(na.Bond, slashAmount)

			if common.RuneAsset().Chain.Equals(common.THORChain) {
				coin := common.NewCoin(common.RuneNative, slashAmount)
				if err := s.keeper.SendFromModuleToModule(ctx, BondName, ReserveName, coin); err != nil {
					ctx.Logger().Error("fail to transfer funds from reserve to asgard", "error", err)
					return fmt.Errorf("fail to transfer funds from reserve to asgard: %w", err)
				}
			} else {
				vaultData, err := s.keeper.GetVaultData(ctx)
				if err != nil {
					return fmt.Errorf("fail to get vault data: %w", err)
				}
				vaultData.TotalReserve = vaultData.TotalReserve.Add(slashAmount)
				if err := s.keeper.SetVaultData(ctx, vaultData); err != nil {
					return fmt.Errorf("fail to save vault data: %w", err)
				}
			}

			return s.keeper.SetNodeAccount(ctx, na)
		}
	}

	return fmt.Errorf("could not find node account with validator address: %s", addr)
}

// LackObserving Slash node accounts that didn't observe a single inbound txn
func (s *SlasherV1) LackObserving(ctx cosmos.Context, constAccessor constants.ConstantValues) error {
	accs, err := s.keeper.GetObservingAddresses(ctx)
	if err != nil {
		return fmt.Errorf("fail to get observing addresses: %w", err)
	}

	if len(accs) == 0 {
		// nobody observed anything, THORNode must of had no input txs within this block
		return nil
	}

	nodes, err := s.keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		return fmt.Errorf("unable to get list of active accounts: %w", err)
	}

	for _, na := range nodes {
		found := false
		for _, addr := range accs {
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

	return nil
}

// LackSigning slash account that fail to sign tx
func (s *SlasherV1) LackSigning(ctx cosmos.Context, constAccessor constants.ConstantValues, mgr Manager) error {
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
		if tx.OutHash.IsEmpty() {
			// Slash node account for not sending funds
			vault, err := s.keeper.GetVault(ctx, tx.VaultPubKey)
			if err != nil {
				ctx.Logger().Error("Unable to get vault", "error", err, "vault pub key", tx.VaultPubKey.String())
				continue
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
				releaseHeight := common.BlockHeight(ctx) + (signingTransPeriod * 2)
				reason := "fail to send yggdrasil transaction"
				if err := s.keeper.SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
					ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
				}
			}

			active, err := s.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				return fmt.Errorf("fail to get active asgard vaults: %w", err)
			}

			vault = active.SelectByMinCoin(tx.Coin.Asset)
			if vault.IsEmpty() {
				ctx.Logger().Error("unable to determine asgard vault to send funds")
				resultErr = fmt.Errorf("unable to determine asgard vault to send funds")
				continue
			}

			// update original tx action in observed tx
			voter, err := s.keeper.GetObservedTxInVoter(ctx, tx.InHash)
			if err != nil {
				ctx.Logger().Error("fail to get observed tx voter", "error", err)
				resultErr = fmt.Errorf("failed to get observed tx voter: %w", err)
				continue
			}

			// check observedTx has done status. Skip if it does already.
			voterTx := voter.GetTx(NodeAccounts{})
			if voterTx.IsDone(len(voter.Actions)) {
				if len(voterTx.OutHashes) > 0 {
					txs.TxArray[i].OutHash = voterTx.OutHashes[0]
				}
				continue
			}

			for i, action := range voter.Actions {
				if action.Equals(*tx) {
					voter.Actions[i].VaultPubKey = vault.PubKey
				}
			}
			s.keeper.SetObservedTxInVoter(ctx, voter)

			if tx.Memo == "" {
				// fetch memo from tx marker
				hash, err := tx.TxHash()
				if err != nil {
					ctx.Logger().Error("fail to get hash", "error", err)
					continue
				}
				marks, err := s.keeper.ListTxMarker(ctx, hash)
				if err != nil {
					ctx.Logger().Error("fail to get markers", "error", err)
					continue
				}
				period := constAccessor.GetInt64Value(constants.SigningTransactionPeriod) * 3
				marks = marks.FilterByMinHeight(common.BlockHeight(ctx) - period)
				mark, _ := marks.Pop()
				tx.Memo = mark.Memo
			}

			// Save the tx to as a new tx, select Asgard to send it this time.
			tx.VaultPubKey = vault.PubKey

			memo, _ := ParseMemo(tx.Memo) // ignore err
			if memo.IsInternal() {
				// there is a different mechanism for rescheduling outbound
				// transactions for migration transactions
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

// slashNodeAccount thorchain keep monitoring the outbound tx from asgard pool
// and yggdrasil pool, usually the txout is triggered by thorchain itself by
// adding an item into the txout array, refer to TxOutItem for the detail, the
// TxOutItem contains a specific coin and amount.  if somehow thorchain
// discover signer send out fund more than the amount specified in TxOutItem,
// it will slash the node account who does that by taking 1.5 * extra fund from
// node account's bond and subsidise the pool that actually lost it.
func (s *SlasherV1) SlashNodeAccount(ctx cosmos.Context, observedPubKey common.PubKey, asset common.Asset, slashAmount cosmos.Uint, mgr Manager) error {
	if slashAmount.IsZero() {
		return nil
	}
	ctx.Logger().Info("slash node account", "observed pub key", observedPubKey.String(), "asset", asset.String(), "amount", slashAmount.String())
	nodeAccount, err := s.keeper.GetNodeAccountByPubKey(ctx, observedPubKey)
	if err != nil {
		return fmt.Errorf("fail to get node account with pubkey(%s), %w", observedPubKey, err)
	}

	if nodeAccount.Status == NodeUnknown {
		return nil
	}

	if asset.IsRune() {
		// If rune, we take 1.5x the amount, and take it from their bond. We
		// put 1/3rd of it into the reserve, and 2/3rds into the pools (but
		// keeping the rune pool balances unchanged)
		amountToReserve := slashAmount.QuoUint64(2)
		// if the diff asset is RUNE , just took 1.5 * diff from their bond
		slashAmount = slashAmount.MulUint64(3).QuoUint64(2)
		nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, slashAmount)
		vaultData, err := s.keeper.GetVaultData(ctx)
		if err != nil {
			return fmt.Errorf("fail to get vault data: %w", err)
		}
		vaultData.TotalReserve = vaultData.TotalReserve.Add(amountToReserve)
		if err := s.keeper.SetVaultData(ctx, vaultData); err != nil {
			return fmt.Errorf("fail to save vault data: %w", err)
		}
		return s.keeper.SetNodeAccount(ctx, nodeAccount)
	}
	pool, err := s.keeper.GetPool(ctx, asset)
	if err != nil {
		return fmt.Errorf("fail to get %s pool : %w", asset, err)
	}
	// thorchain doesn't even have a pool for the asset, or the pool had been
	// suspended, then who cares
	if pool.IsEmpty() || pool.Status == PoolSuspended {
		return nil
	}
	runeValue := pool.AssetValueInRune(slashAmount).MulUint64(3).QuoUint64(2)
	pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, slashAmount)
	pool.BalanceRune = pool.BalanceRune.Add(runeValue)
	nodeAccount.Bond = common.SafeSub(nodeAccount.Bond, runeValue)
	if err := s.keeper.SetPool(ctx, pool); err != nil {
		return fmt.Errorf("fail to save %s pool: %w", asset, err)
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
	if err := mgr.EventMgr().EmitSlashEvent(ctx, eventSlash); err != nil {
		return fmt.Errorf("fail to emit slash event: %w", err)
	}

	return s.keeper.SetNodeAccount(ctx, nodeAccount)
}

// IncSlashPoints will increase the given account's slash points
func (s *SlasherV1) IncSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		if err := s.keeper.IncNodeAccountSlashPoints(ctx, addr, point); err != nil {
			ctx.Logger().Error("fail to increase node account slash point", "error", err, "address", addr.String())
		}
	}
}

// DecSlashPoints will decrease the given account's slash points
func (s *SlasherV1) DecSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		if err := s.keeper.DecNodeAccountSlashPoints(ctx, addr, point); err != nil {
			ctx.Logger().Error("fail to decrease node account slash point", "error", err, "address", addr.String())
		}
	}
}
