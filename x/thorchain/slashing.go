package thorchain

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/constants"
)

// Slash node accounts that didn't observe a single inbound txn
func slashForObservingAddresses(ctx sdk.Context, keeper Keeper) {
	accs := keeper.GetObservingAddresses(ctx)

	if len(accs) == 0 {
		// nobody observed anything, THORNode must of had no input txs within this
		// block
		return
	}

	nodes, err := keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		ctx.Logger().Error("Unable to get list of active accounts", err)
		return
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
			na.SlashPoints += constants.LackOfObservationPenalty
			keeper.SetNodeAccount(ctx, na)
		}
	}

	// clear our list of observing addresses
	keeper.ClearObservingAddresses(ctx)

	return
}

func slashForNotSigning(ctx sdk.Context, keeper Keeper, txOutStore *TxOutStore) {
	incomplete, err := keeper.GetIncompleteEvents(ctx)
	if err != nil {
		ctx.Logger().Error("Unable to get list of active accounts", err)
		return
	}

	for _, evt := range incomplete {
		// NOTE: not checking the event type because all non-swap/unstake/etc
		// are completed immediately.
		fmt.Printf("%d %d %d\n", evt.Height, constants.SigningTransactionPeriod, ctx.BlockHeight())
		if evt.Height+constants.SigningTransactionPeriod < ctx.BlockHeight() {
			txs, err := keeper.GetTxOut(ctx, uint64(evt.Height))
			if err != nil {
				ctx.Logger().Error("Unable to get tx out list", err)
				continue
			}

			for i, tx := range txs.TxArray {
				if tx.InHash.Equals(evt.InTx.ID) && tx.OutHash.IsEmpty() {
					// Slash our node account for not sending funds
					txs.TxArray[i].OutHash = common.BlankTxID
					na, err := keeper.GetNodeAccountByPubKey(ctx, tx.PoolAddress)
					if err != nil {
						ctx.Logger().Error("Unable to get node account", err)
						continue
					}
					na.SlashPoints += constants.SigningTransactionPeriod * 2
					keeper.SetNodeAccount(ctx, na)

					// Save the tx to as a new tx, select Asgard to send it this time.
					// Set the pool address to empty, it will overwrite it with the
					// current Asgard vault
					tx.PoolAddress = common.EmptyPubKey
					// TODO: this creates a second tx out for this inTx, which
					// means the event will never be completed because only one
					// of the two out tx will occur.
					txOutStore.AddTxOutItem(ctx, keeper, tx, true)
				}
			}

			keeper.SetTxOut(ctx, txs)
		}
	}
}
