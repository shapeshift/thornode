package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// SwitchHandler is to handle Switch message
type SwitchHandler struct {
	keeper Keeper
	mgr    Manager
}

// NewSwitchHandler create new instance of SwitchHandler
func NewSwitchHandler(keeper Keeper, mgr Manager) SwitchHandler {
	return SwitchHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run it the main entry point to execute Switch logic
func (h SwitchHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgSwitch)
	if !ok {
		return errInvalidMessage.Result()
	}
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("msg switch failed validation", "error", err)
		return err.Result()
	}
	return h.handle(ctx, msg, version)
}

func (h SwitchHandler) validate(ctx cosmos.Context, msg MsgSwitch, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		return errBadVersion
	}
}

func (h SwitchHandler) validateV1(ctx cosmos.Context, msg MsgSwitch) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(notAuthorized.Error())
	}

	return nil
}

func (h SwitchHandler) handle(ctx cosmos.Context, msg MsgSwitch, version semver.Version) cosmos.Result {
	ctx.Logger().Info("handleMsgSwitch request", "destination address", msg.Destination.String())
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion.Result()
	}
}

func (h SwitchHandler) handleV1(ctx cosmos.Context, msg MsgSwitch, version semver.Version) cosmos.Result {
	bank := h.keeper.CoinKeeper()

	vaultData, err := h.keeper.GetVaultData(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get vault data", "error", err)
		return cosmos.ErrInternal("fail to get vault data").Result()
	}

	if msg.Tx.Coins[0].IsNative() {
		coin, err := common.NewCoin(common.RuneNative, msg.Tx.Coins[0].Amount).Native()
		if err != nil {
			ctx.Logger().Error("fail to get native coin", "error", err)
			return cosmos.ErrInternal("fail to get native coin").Result()
		}

		// ensure we have enough BEP2 rune assets to fulfill the request
		if vaultData.TotalBEP2Rune.LT(msg.Tx.Coins[0].Amount) {
			ctx.Logger().Error("not enough funds in the vault", "error", err)
			return cosmos.ErrInternal("not enough funds in the vault").Result()
		}

		addr, err := cosmos.AccAddressFromBech32(msg.Tx.FromAddress.String())
		if err != nil {
			ctx.Logger().Error("fail to parse thor address", "error", err)
			return cosmos.ErrInternal("fail to parse thor address").Result()
		}

		if !bank.HasCoins(ctx, addr, cosmos.NewCoins(coin)) {
			ctx.Logger().Error("insufficient funds", "error", err)
			return cosmos.ErrInternal("insufficient funds").Result()
		}
		if _, err := bank.SubtractCoins(ctx, addr, cosmos.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to burn native rune coins", "error", err)
			return cosmos.ErrInternal("fail to burn native rune coins").Result()
		}

		vaultData.TotalBEP2Rune = common.SafeSub(vaultData.TotalBEP2Rune, msg.Tx.Coins[0].Amount)

		toi := &TxOutItem{
			Chain:     common.RuneAsset().Chain,
			InHash:    msg.Tx.ID,
			ToAddress: msg.Destination,
			Coin:      common.NewCoin(common.RuneAsset(), msg.Tx.Coins[0].Amount),
		}
		ok, err := h.mgr.TxOutStore().TryAddTxOutItem(ctx, h.mgr, toi)
		if err != nil {
			ctx.Logger().Error("fail to add outbound tx", "error", err)
			return cosmos.ErrInternal(fmt.Errorf("fail to add outbound tx: %w", err).Error()).Result()
		}
		if !ok {
			return cosmos.NewError(DefaultCodespace, CodeFailAddOutboundTx, "prepare outbound tx not successful").Result()
		}
	} else {
		coin, err := common.NewCoin(common.RuneNative, msg.Tx.Coins[0].Amount).Native()
		if err != nil {
			ctx.Logger().Error("fail to get native coin", "error", err)
			return cosmos.ErrInternal("fail to get native coin").Result()
		}

		addr, err := cosmos.AccAddressFromBech32(msg.Destination.String())
		if err != nil {
			ctx.Logger().Error("fail to parse thor address", "error", err)
			return cosmos.ErrInternal("fail to parse thor address").Result()
		}
		if _, err := bank.AddCoins(ctx, addr, cosmos.NewCoins(coin)); err != nil {
			ctx.Logger().Error("fail to mint native rune coins", "error", err)
			return cosmos.ErrInternal("fail to mint native rune coins").Result()
		}
		vaultData.TotalBEP2Rune = vaultData.TotalBEP2Rune.Add(msg.Tx.Coins[0].Amount)
	}

	if err := h.keeper.SetVaultData(ctx, vaultData); err != nil {
		ctx.Logger().Error("fail to set vault data", "error", err)
		return cosmos.ErrInternal("fail to set vault data").Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}
