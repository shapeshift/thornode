package thorchain

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	ckeys "github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/hashicorp/go-multierror"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

const (
	// folder name for thorchain thorcli
	thorchainCliFolderName = `.thorcli`
)

func refundTx(ctx cosmos.Context, tx ObservedTx, mgr Manager, keeper Keeper, constAccessor constants.ConstantValues, refundCode uint32, refundReason string) error {
	// If THORNode recognize one of the coins, and therefore able to refund
	// withholding fees, refund all coins.

	addEvent := func(refundCoins common.Coins) error {
		eventRefund := NewEventRefund(refundCode, refundReason, tx.Tx, common.NewFee(common.Coins{}, cosmos.ZeroUint()))
		status := EventSuccess
		if len(refundCoins) > 0 {
			// create a new TX based on the coins thorchain refund , some of the coins thorchain doesn't refund
			// coin thorchain doesn't have pool with , likely airdrop
			newTx := common.NewTx(tx.Tx.ID, tx.Tx.FromAddress, tx.Tx.ToAddress, tx.Tx.Coins, tx.Tx.Gas, tx.Tx.Memo)
			transactionFee := constAccessor.GetInt64Value(constants.TransactionFee)
			fee := getFee(tx.Tx.Coins, refundCoins, transactionFee)
			eventRefund = NewEventRefund(refundCode, refundReason, newTx, fee)
			status = EventPending
		}
		if err := mgr.EventMgr().EmitRefundEvent(ctx, keeper, eventRefund, status); err != nil {
			return fmt.Errorf("fail to emit refund event: %w", err)
		}
		return nil
	}

	// for THORChain transactions, create the event before we txout. For other
	// chains, do it after. The reason for this is we need to make sure the
	// first event (refund) is created, before we create the outbound events
	// (second). Because its THORChain, its safe to assume all the coins are
	// safe to send back. Where as for external coins, we cannot make this
	// assumption (ie coins we don't have pools for and therefore, don't know
	// the value of it relative to rune)
	if tx.Tx.Chain.Equals(common.THORChain) {
		if err := addEvent(tx.Tx.Coins); err != nil {
			return err
		}
	}

	refundCoins := make(common.Coins, 0)
	for _, coin := range tx.Tx.Coins {
		pool, err := keeper.GetPool(ctx, coin.Asset)
		if err != nil {
			return fmt.Errorf("fail to get pool: %w", err)
		}

		if coin.Asset.IsRune() || !pool.BalanceRune.IsZero() {
			toi := &TxOutItem{
				Chain:       coin.Asset.Chain,
				InHash:      tx.Tx.ID,
				ToAddress:   tx.Tx.FromAddress,
				VaultPubKey: tx.ObservedPubKey,
				Coin:        coin,
				Memo:        NewRefundMemo(tx.Tx.ID).String(),
			}

			success, err := mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, toi)
			if err != nil {
				return fmt.Errorf("fail to prepare outbund tx: %w", err)
			}
			if success {
				refundCoins = append(refundCoins, toi.Coin)
			}
		}
		// Zombie coins are just dropped.
	}
	if !tx.Tx.Chain.Equals(common.THORChain) {
		if err := addEvent(refundCoins); err != nil {
			return err
		}
	}

	return nil
}

func getFee(input, output common.Coins, transactionFee int64) common.Fee {
	var fee common.Fee
	assetTxCount := 0
	for _, out := range output {
		if !out.Asset.IsRune() {
			assetTxCount++
		}
	}
	for _, in := range input {
		outCoin := common.NoCoin
		for _, out := range output {
			if out.Asset.Equals(in.Asset) {
				outCoin = out
				break
			}
		}
		if outCoin.IsEmpty() {
			if !in.Amount.IsZero() {
				fee.Coins = append(fee.Coins, common.NewCoin(in.Asset, in.Amount))
			}
		} else {
			if !in.Amount.Sub(outCoin.Amount).IsZero() {
				fee.Coins = append(fee.Coins, common.NewCoin(in.Asset, in.Amount.Sub(outCoin.Amount)))
			}
		}
	}
	fee.PoolDeduct = cosmos.NewUint(uint64(transactionFee) * uint64(assetTxCount))
	return fee
}

func subsidizePoolWithSlashBond(ctx cosmos.Context, keeper Keeper, ygg Vault, yggTotalStolen, slashRuneAmt cosmos.Uint) error {
	// Thorchain did not slash the node account
	if slashRuneAmt.IsZero() {
		return nil
	}
	stolenRUNE := ygg.GetCoin(common.RuneAsset()).Amount
	slashRuneAmt = common.SafeSub(slashRuneAmt, stolenRUNE)
	yggTotalStolen = common.SafeSub(yggTotalStolen, stolenRUNE)
	type fund struct {
		stolenAsset   cosmos.Uint
		subsidiseRune cosmos.Uint
	}
	// here need to use a map to hold on to the amount of RUNE need to be subsidized to each pool
	// reason being , if ygg pool has both RUNE and BNB coin left, these two coin share the same pool
	// which is BNB pool , if add the RUNE directly back to pool , it will affect BNB price , which will affect the result
	subsidizeAmounts := make(map[common.Asset]fund)
	for _, coin := range ygg.Coins {
		asset := coin.Asset
		if coin.Asset.IsRune() {
			// when the asset is RUNE, thorchain don't need to update the RUNE balance on pool
			continue
		}
		f, ok := subsidizeAmounts[asset]
		if !ok {
			f = fund{
				stolenAsset:   cosmos.ZeroUint(),
				subsidiseRune: cosmos.ZeroUint(),
			}
		}

		pool, err := keeper.GetPool(ctx, asset)
		if err != nil {
			return err
		}
		f.stolenAsset = f.stolenAsset.Add(coin.Amount)
		runeValue := pool.AssetValueInRune(coin.Amount)
		// the amount of RUNE thorchain used to subsidize the pool is calculate by ratio
		// slashRune * (stealAssetRuneValue /totalStealAssetRuneValue)
		subsidizeAmt := slashRuneAmt.Mul(runeValue).Quo(yggTotalStolen)
		f.subsidiseRune = f.subsidiseRune.Add(subsidizeAmt)
		subsidizeAmounts[asset] = f
	}

	for asset, f := range subsidizeAmounts {
		pool, err := keeper.GetPool(ctx, asset)
		if err != nil {
			return err
		}
		pool.BalanceRune = pool.BalanceRune.Add(f.subsidiseRune)
		pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, f.stolenAsset)

		if err := keeper.SetPool(ctx, pool); err != nil {
			return fmt.Errorf("fail to save pool: %w", err)
		}
	}
	return nil
}

// getTotalYggValueInRune will go through all the coins in ygg , and calculate the total value in RUNE
// return value will be totalValueInRune,error
func getTotalYggValueInRune(ctx cosmos.Context, keeper Keeper, ygg Vault) (cosmos.Uint, error) {
	yggRune := cosmos.ZeroUint()
	for _, coin := range ygg.Coins {
		if coin.Asset.IsRune() {
			yggRune = yggRune.Add(coin.Amount)
		} else {
			pool, err := keeper.GetPool(ctx, coin.Asset)
			if err != nil {
				return cosmos.ZeroUint(), err
			}
			yggRune = yggRune.Add(pool.AssetValueInRune(coin.Amount))
		}
	}
	return yggRune, nil
}

func refundBond(ctx cosmos.Context, tx common.Tx, nodeAcc NodeAccount, keeper Keeper, mgr Manager) error {
	if nodeAcc.Status == NodeActive {
		ctx.Logger().Info("node still active , cannot refund bond", "node address", nodeAcc.NodeAddress, "node pub key", nodeAcc.PubKeySet.Secp256k1)
		return nil
	}

	ygg := Vault{}
	if keeper.VaultExists(ctx, nodeAcc.PubKeySet.Secp256k1) {
		var err error
		ygg, err = keeper.GetVault(ctx, nodeAcc.PubKeySet.Secp256k1)
		if err != nil {
			return err
		}
		if !ygg.IsYggdrasil() {
			return errors.New("this is not a Yggdrasil vault")
		}
	}

	// Calculate total value (in rune) the Yggdrasil pool has
	yggRune, err := getTotalYggValueInRune(ctx, keeper, ygg)
	if err != nil {
		return fmt.Errorf("fail to get total ygg value in RUNE: %w", err)
	}

	if nodeAcc.Bond.LT(yggRune) {
		ctx.Logger().Error(fmt.Sprintf("Node Account (%s) left with more funds in their Yggdrasil vault than their bond's value (%s / %s)", nodeAcc.NodeAddress, yggRune, nodeAcc.Bond))
	}
	// slashing 1.5 * yggdrasil remains
	slashRune := yggRune.MulUint64(3).QuoUint64(2)
	bondBeforeSlash := nodeAcc.Bond
	nodeAcc.Bond = common.SafeSub(nodeAcc.Bond, slashRune)

	if !nodeAcc.Bond.IsZero() {
		active, err := keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			ctx.Logger().Error("fail to get active vaults", "error", err)
			return err
		}

		vault := active.SelectByMinCoin(common.RuneAsset())
		if vault.IsEmpty() {
			return fmt.Errorf("unable to determine asgard vault to send funds")
		}

		bondEvent := NewEventBond(nodeAcc.Bond, BondReturned, tx)
		if err := mgr.EventMgr().EmitBondEvent(ctx, keeper, bondEvent); err != nil {
			return fmt.Errorf("fail to emit bond event: %w", err)
		}

		refundAddress := nodeAcc.BondAddress
		if common.RuneAsset().Chain.Equals(common.THORChain) {
			refundAddress = common.Address(nodeAcc.NodeAddress.String())
		}

		// refund bond
		txOutItem := &TxOutItem{
			Chain:       common.RuneAsset().Chain,
			ToAddress:   refundAddress,
			VaultPubKey: vault.PubKey,
			InHash:      tx.ID,
			Coin:        common.NewCoin(common.RuneAsset(), nodeAcc.Bond),
			ModuleName:  BondName,
		}
		_, err = mgr.TxOutStore().TryAddTxOutItem(ctx, mgr, txOutItem)
		if err != nil {
			return fmt.Errorf("fail to add outbound tx: %w", err)
		}
	} else {
		// if it get into here that means the node account doesn't have any bond left after slash.
		// which means the real slashed RUNE could be the bond they have before slash
		slashRune = bondBeforeSlash
	}

	nodeAcc.Bond = cosmos.ZeroUint()
	// disable the node account
	nodeAcc.UpdateStatus(NodeDisabled, ctx.BlockHeight())
	if err := keeper.SetNodeAccount(ctx, nodeAcc); err != nil {
		ctx.Logger().Error(fmt.Sprintf("fail to save node account(%s)", nodeAcc), "error", err)
		return err
	}
	if err := subsidizePoolWithSlashBond(ctx, keeper, ygg, yggRune, slashRune); err != nil {
		ctx.Logger().Error("fail to subsidize pool with slashed bond", "error", err)
		return err
	}
	// delete the ygg vault, there is nothing left in the ygg vault
	if !ygg.HasFunds() {
		return keeper.DeleteVault(ctx, ygg.PubKey)
	}
	return nil
}

// Checks if the observed vault pubkey is a valid asgard or ygg vault
func isCurrentVaultPubKey(ctx cosmos.Context, keeper Keeper, tx ObservedTx) bool {
	return keeper.VaultExists(ctx, tx.ObservedPubKey)
}

func isSignedByActiveNodeAccounts(ctx cosmos.Context, keeper Keeper, signers []cosmos.AccAddress) bool {
	if len(signers) == 0 {
		return false
	}
	supplier := keeper.Supply()
	for _, signer := range signers {
		if signer.Equals(supplier.GetModuleAddress(AsgardName)) {
			continue
		}
		nodeAccount, err := keeper.GetNodeAccount(ctx, signer)
		if err != nil {
			ctx.Logger().Error("unauthorized account", "address", signer.String(), "error", err)
			return false
		}
		if nodeAccount.IsEmpty() {
			ctx.Logger().Error("unauthorized account", "address", signer.String())
			return false
		}
		if nodeAccount.Status != NodeActive {
			ctx.Logger().Error("unauthorized account, node account not active", "address", signer.String(), "status", nodeAccount.Status)
			return false
		}
	}
	return true
}

func updateEventStatus(ctx cosmos.Context, keeper Keeper, eventID int64, txs common.Txs, eventStatus EventStatus) error {
	event, err := keeper.GetEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("fail to get event: %w", err)
	}

	// if the event is already successful, don't append more transactions
	if event.Status == EventSuccess {
		return nil
	}

	ctx.Logger().Info(fmt.Sprintf("set event to %s,eventID (%d) , txs:%s", eventStatus, eventID, txs))

	for _, item := range txs {
		duplicate := false
		for i := 0; i < len(event.OutTxs); i++ {
			if event.OutTxs[i].Equals(item) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			event.OutTxs = append(event.OutTxs, item)
		}
	}
	if eventStatus == RefundStatus {
		// we need to check we refunded all the coins that need to be refunded from in tx
		// before updating status to complete, we use the count of voter actions to check
		voter, err := keeper.GetObservedTxVoter(ctx, event.InTx.ID)
		if err != nil {
			return fmt.Errorf("fail to get observed tx voter: %w", err)
		}
		if len(voter.Actions) <= len(event.OutTxs) {
			event.Status = eventStatus
		}
	} else {
		event.Status = eventStatus
	}
	return keeper.UpsertEvent(ctx, event)
}

func updateEventFee(ctx cosmos.Context, keeper Keeper, txID common.TxID, fee common.Fee) error {
	ctx.Logger().Info("update event fee txid", "tx", txID.String())
	eventIDs, err := keeper.GetEventsIDByTxHash(ctx, txID)
	if err != nil {
		if err == ErrEventNotFound {
			ctx.Logger().Error(fmt.Sprintf("could not find the event(%s)", txID))
			return nil
		}
		return fmt.Errorf("fail to get event id: %w", err)
	}
	if len(eventIDs) == 0 {
		return errors.New("no event found")
	}
	// There are two events for double swap with the same the same txID. Only the second one has fee
	eventID := eventIDs[len(eventIDs)-1]
	event, err := keeper.GetEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("fail to get event: %w", err)
	}

	ctx.Logger().Info(fmt.Sprintf("Update fee for event %d, fee:%s", eventID, fee))
	for _, feeCoin := range fee.Coins {
		if !feeCoin.IsEmpty() {
			event.Fee.Coins = append(event.Fee.Coins, feeCoin)
		}
	}
	event.Fee.PoolDeduct = event.Fee.PoolDeduct.Add(fee.PoolDeduct)
	return keeper.UpsertEvent(ctx, event)
}

func completeEvents(ctx cosmos.Context, keeper Keeper, txID common.TxID, txs common.Txs, eventStatus EventStatus) error {
	ctx.Logger().Info(fmt.Sprintf("txid(%s)", txID))
	eventIDs, err := keeper.GetPendingEventID(ctx, txID)
	if err != nil {
		if err == ErrEventNotFound {
			ctx.Logger().Error(fmt.Sprintf("could not find the event(%s)", txID))
			return nil
		}
		return fmt.Errorf("fail to get pending event id: %w", err)
	}
	for _, item := range eventIDs {
		if err := updateEventStatus(ctx, keeper, item, txs, eventStatus); err != nil {
			return fmt.Errorf("fail to set event(%d) to %s: %w", item, eventStatus, err)
		}
	}
	return nil
}

func enableNextPool(ctx cosmos.Context, keeper Keeper, eventManager EventManager) error {
	var pools []Pool
	iterator := keeper.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().UnmarshalBinaryBare(iterator.Value(), &pool); err != nil {
			return err
		}

		if pool.Status == PoolBootstrap && !pool.BalanceAsset.IsZero() && !pool.BalanceRune.IsZero() {
			pools = append(pools, pool)
		}
	}

	if len(pools) == 0 {
		return nil
	}

	pool := pools[0]
	for _, p := range pools {
		// find the pool that has most RUNE, also exclude those pool that doesn't have asset
		if pool.BalanceRune.LT(p.BalanceRune) {
			pool = p
		}
	}

	poolEvt := NewEventPool(pool.Asset, PoolEnabled)
	if err := eventManager.EmitPoolEvent(ctx, keeper, common.BlankTxID, EventSuccess, poolEvt); err != nil {
		return fmt.Errorf("fail to emit pool event: %w", err)
	}

	pool.Status = PoolEnabled
	return keeper.SetPool(ctx, pool)
}

func wrapError(ctx cosmos.Context, err error, wrap string) error {
	err = fmt.Errorf("%s: %w", wrap, err)
	ctx.Logger().Error(err.Error())
	return multierror.Append(errInternal, err)
}

func AddGasFees(ctx cosmos.Context, keeper Keeper, tx ObservedTx, gasManager GasManager) error {
	if len(tx.Tx.Gas) == 0 {
		return nil
	}

	// update state with new gas info
	if len(tx.Tx.Coins) > 0 {
		gasAsset := tx.Tx.Coins[0].Asset.Chain.GetGasAsset()
		gasInfo, err := keeper.GetGas(ctx, gasAsset)
		if err == nil {
			gasInfo = common.UpdateGasPrice(tx.Tx, gasAsset, gasInfo)
			if gasInfo != nil {
				keeper.SetGas(ctx, gasAsset, gasInfo)
			} else {
				ctx.Logger().Error(fmt.Sprintf("fail to update gas price for chain: %s", gasAsset))
			}
		}
	}

	gasManager.AddGasAsset(tx.Tx.Gas)

	// Subtract from the vault
	if keeper.VaultExists(ctx, tx.ObservedPubKey) {
		vault, err := keeper.GetVault(ctx, tx.ObservedPubKey)
		if err != nil {
			return err
		}

		vault.SubFunds(tx.Tx.Gas.ToCoins())

		if err := keeper.SetVault(ctx, vault); err != nil {
			return err
		}
	}
	return nil
}

type KeybaseStore struct {
	Keybase      ckeys.Keybase
	SignerName   string
	SignerPasswd string
}

func signerCreds() (string, string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Username: \n")
	username, _ := reader.ReadString('\n')

	fmt.Print("Enter Password: \n")
	// TODO: currently using an insecure means of getting the password, we may
	// be echo'ing it now. The following commented out code supposed to fix
	// that but currently causes a "inappropriate ioctl for device" error.
	// Go-ethereum has already solved this problem, but hasn't released it yet
	// (5/22/20), but should within a month. When it drops, update our version
	// of go-ethereum to later than 1.9.14
	// (https://github.com/ethereum/go-ethereum/pull/20960).
	/*
		bytePassword err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		password := string(bytePassword)
	*/
	password, _ := reader.ReadString('\n')

	return strings.TrimSpace(username), strings.TrimSpace(password)
}

// getKeybase will create an instance of Keybase
func getKeybase(thorchainHome string) (KeybaseStore, error) {
	username, password := signerCreds()

	buf := bytes.NewBufferString(password)
	// the library used by keyring is using ReadLine , which expect a new line
	buf.WriteByte('\n')

	cliDir := thorchainHome
	if len(thorchainHome) == 0 {
		usr, err := user.Current()
		if err != nil {
			return KeybaseStore{}, fmt.Errorf("fail to get current user,err:%w", err)
		}
		cliDir = filepath.Join(usr.HomeDir, thorchainCliFolderName)
	}

	kb, err := ckeys.NewKeyring(cosmos.KeyringServiceName(), ckeys.BackendFile, cliDir, buf)
	return KeybaseStore{
		SignerName:   username,
		SignerPasswd: password,
		Keybase:      kb,
	}, err
}
