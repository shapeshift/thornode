package thorchain

import (
	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/x/gov/types"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type SendHandler struct {
	keeper Keeper
	mgr    Manager
}

func NewSendHandler(keeper Keeper, mgr Manager) SendHandler {
	return SendHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h SendHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgSend)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return nil, err
	}
	return h.handle(ctx, msg, version, constAccessor)
}

func (h SendHandler) validate(ctx cosmos.Context, msg MsgSend, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errInvalidVersion
	}
}

func (h SendHandler) validateV1(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return err
	}

	return nil
}

func (h SendHandler) handle(ctx cosmos.Context, msg MsgSend, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSend", "from", msg.FromAddress, "to", msg.ToAddress, "coins", msg.Amount)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return nil, errBadVersion
	}
}

func (h SendHandler) handleV1(ctx cosmos.Context, msg MsgSend, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	banker := h.keeper.CoinKeeper()
	supplier := h.keeper.Supply()
	// TODO: this shouldn't be tied to swaps, and should be cheaper. But
	// TransactionFee will be fine for now.
	transactionFee := constAccessor.GetInt64Value(constants.TransactionFee)

	gasFee, err := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(transactionFee))).Native()
	if err != nil {
		return nil, ErrInternal(err, "fail to get gas fee")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(msg.Amount...)
	if !banker.HasCoins(ctx, msg.FromAddress, totalCoins) {
		ctx.Logger().Error("insufficient funds", "error", err)
		return nil, cosmos.ErrInsufficientCoins("insufficient funds")
	}

	// send gas to reserve
	sdkErr := supplier.SendCoinsFromAccountToModule(ctx, msg.FromAddress, ReserveName, cosmos.NewCoins(gasFee))
	if sdkErr != nil {
		ctx.Logger().Error("unable to send gas to reserve", "error", sdkErr)
		return nil, sdkErr
	}

	sdkErr = banker.SendCoins(ctx, msg.FromAddress, msg.ToAddress, msg.Amount)
	if sdkErr != nil {
		return nil, sdkErr
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent(
			cosmos.EventTypeMessage,
			cosmos.NewAttribute(cosmos.AttributeKeyModule, types.AttributeValueCategory),
		),
	)

	return &cosmos.Result{}, nil
}
