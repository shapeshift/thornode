package thorchain

import (
	stdErrors "errors"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// YggdrasilHandler is to process yggdrasil messages
// When thorchain fund yggdrasil pool , observer should observe two transactions
// 1. outbound tx from asgard vault
// 2. inbound tx to yggdrasil vault
// when yggdrasil pool return fund , observer should observe two transactions as well
// 1. outbound tx from yggdrasil vault
// 2. inbound tx to asgard vault
type YggdrasilHandler struct {
	keeper Keeper
	mgr    Manager
}

// NewYggdrasilHandler create a new Yggdrasil handler
func NewYggdrasilHandler(keeper Keeper, mgr Manager) YggdrasilHandler {
	return YggdrasilHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run execute the logic in Yggdrasil Handler
func (h YggdrasilHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgYggdrasil)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return nil, err
	}
	return h.handle(ctx, msg, version, constAccessor)
}

func (h YggdrasilHandler) validate(ctx cosmos.Context, msg MsgYggdrasil, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errBadVersion
}

func (h YggdrasilHandler) validateV1(ctx cosmos.Context, msg MsgYggdrasil) error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return err
	}
	return nil
}

func (h YggdrasilHandler) handle(ctx cosmos.Context, msg MsgYggdrasil, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgYggdrasil", "pubkey", msg.PubKey.String(), "add_funds", msg.AddFunds, "coins", msg.Coins)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return nil, errBadVersion
	}
}

func (h YggdrasilHandler) slash(ctx cosmos.Context, version semver.Version, pk common.PubKey, coins common.Coins) error {
	var returnErr error
	slasher, err := NewSlasher(h.keeper, version, h.mgr)
	if err != nil {
		return fmt.Errorf("fail to create new slasher,error:%w", err)
	}
	for _, c := range coins {
		if err := slasher.SlashNodeAccount(ctx, pk, c.Asset, c.Amount); err != nil {
			ctx.Logger().Error("fail to slash account", "error", err)
			returnErr = err
		}
	}
	return returnErr
}

func (h YggdrasilHandler) handleV1(ctx cosmos.Context, msg MsgYggdrasil, version semver.Version) (*cosmos.Result, error) {
	// update txOut record with our TxID that sent funds out of the pool
	txOut, err := h.keeper.GetTxOut(ctx, msg.BlockHeight)
	if err != nil {
		ctx.Logger().Error("unable to get txOut record", "error", err)
		return nil, cosmos.ErrUnknownRequest(err.Error())
	}

	shouldSlash := true
	for i, tx := range txOut.TxArray {
		// yggdrasil is the memo used by thorchain to identify fund migration
		// to a yggdrasil vault.
		// it use yggdrasil+/-:{block height} to mark a tx out caused by vault
		// rotation
		// this type of tx out is special , because it doesn't have relevant tx
		// in to trigger it, it is trigger by thorchain itself.
		fromAddress, _ := tx.VaultPubKey.GetAddress(tx.Chain)
		if tx.InHash.Equals(common.BlankTxID) &&
			tx.OutHash.IsEmpty() &&
			tx.ToAddress.Equals(msg.Tx.ToAddress) &&
			fromAddress.Equals(msg.Tx.FromAddress) {

			// only need to check the coin if yggdrasil+
			if msg.AddFunds && !msg.Tx.Coins.Contains(tx.Coin) {
				continue
			}

			txOut.TxArray[i].OutHash = msg.Tx.ID
			shouldSlash = false

			if err := h.keeper.SetTxOut(ctx, txOut); nil != err {
				ctx.Logger().Error("fail to save tx out", "error", err)
			}

			break
		}
	}

	if shouldSlash {
		if err := h.slash(ctx, version, msg.PubKey, msg.Tx.Coins); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	vault, err := h.keeper.GetVault(ctx, msg.PubKey)
	if err != nil && !stdErrors.Is(err, ErrVaultNotFound) {
		ctx.Logger().Error("fail to get yggdrasil", "error", err)
		return nil, err
	}
	if len(vault.Type) == 0 {
		vault.Status = ActiveVault
		vault.Type = YggdrasilVault
	}

	h.keeper.SetLastSignedHeight(ctx, msg.BlockHeight)

	if msg.AddFunds {
		return h.handleYggdrasilFund(ctx, msg, vault)
	}
	return h.handleYggdrasilReturn(ctx, msg, vault, version)
}

func (h YggdrasilHandler) handleYggdrasilFund(ctx cosmos.Context, msg MsgYggdrasil, vault Vault) (*cosmos.Result, error) {
	if vault.Type == AsgardVault {
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("asgard_fund_yggdrasil",
				cosmos.NewAttribute("pubkey", vault.PubKey.String()),
				cosmos.NewAttribute("coins", msg.Coins.String()),
				cosmos.NewAttribute("tx", msg.Tx.ID.String())))
	}
	if vault.Type == YggdrasilVault {
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("yggdrasil_receive_fund",
				cosmos.NewAttribute("pubkey", vault.PubKey.String()),
				cosmos.NewAttribute("coins", msg.Coins.String()),
				cosmos.NewAttribute("tx", msg.Tx.ID.String())))
	}
	// Yggdrasil usually comes from Asgard , Asgard --> Yggdrasil
	// It will be an outbound tx from Asgard pool , and it will be an Inbound tx form Yggdrasil pool
	// incoming fund will be added to Vault as part of ObservedTxInHandler
	// Yggdrasil handler doesn't need to do anything
	return &cosmos.Result{}, nil
}

func (h YggdrasilHandler) handleYggdrasilReturn(ctx cosmos.Context, msg MsgYggdrasil, vault Vault, version semver.Version) (*cosmos.Result, error) {
	// observe an outbound tx from yggdrasil vault
	if vault.Type == YggdrasilVault {
		asgardVaults, err := h.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, ErrInternal(err, "unable to get asgard vaults")
		}
		isAsgardReceipient, err := asgardVaults.HasAddress(msg.Tx.Chain, msg.Tx.ToAddress)
		if err != nil {
			ctx.Logger().Error(fmt.Sprintf("unable to determinate whether %s is an Asgard vault", msg.Tx.ToAddress), "error", err)
			return nil, ErrInternal(err, "unable to check recipient against active Asgards")
		}

		if !isAsgardReceipient {
			// not sending to asgard , slash the node account
			if err := h.slash(ctx, version, msg.PubKey, msg.Tx.Coins); err != nil {
				return nil, ErrInternal(err, "fail to slash account for sending fund to a none asgard vault using yggdrasil-")
			}
		}

		na, err := h.keeper.GetNodeAccountByPubKey(ctx, msg.PubKey)
		if err != nil {
			ctx.Logger().Error("unable to get node account", "error", err)
			return nil, err
		}
		if na.Status == NodeActive {
			// node still active , no refund bond
			return nil, nil
		}

		if !vault.HasFunds() {
			if err := refundBond(ctx, msg.Tx, na, h.keeper, h.mgr); err != nil {
				ctx.Logger().Error("fail to refund bond", "error", err)
				return nil, err
			}
		}
		return &cosmos.Result{}, nil
	}

	// when vault.Type is asgard, that means this tx is observed on an asgard pool and it is an inbound tx
	if vault.Type == AsgardVault {
		// Yggdrasil return fund back to Asgard
		ctx.EventManager().EmitEvent(
			cosmos.NewEvent("yggdrasil_return",
				cosmos.NewAttribute("pubkey", vault.PubKey.String()),
				cosmos.NewAttribute("coins", msg.Coins.String()),
				cosmos.NewAttribute("tx", msg.Tx.ID.String())))
	}
	return nil, nil
}
