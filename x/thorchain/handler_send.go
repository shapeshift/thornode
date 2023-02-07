package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/x/gov/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// SendHandler handle MsgSend
type SendHandler struct {
	mgr Manager
}

// NewSendHandler create a new instance of SendHandler
func NewSendHandler(mgr Manager) SendHandler {
	return SendHandler{
		mgr: mgr,
	}
}

// Run the main entry point to process MsgSend
func (h SendHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSend)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgSend failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgSend", "error", err)
	}
	return result, err
}

func (h SendHandler) validate(ctx cosmos.Context, msg MsgSend) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("1.87.0")) {
		return h.validateV87(ctx, msg)
	} else if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h SendHandler) validateV87(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// disallow sends to modules, they should only be interacted with via deposit messages
	if msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(AsgardName)) ||
		msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(BondName)) ||
		msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(ReserveName)) ||
		msg.ToAddress.Equals(h.mgr.Keeper().GetModuleAccAddress(ModuleName)) {
		return errors.New("cannot use MsgSend for Module transactions, use MsgDeposit instead")
	}

	return nil
}

func (h SendHandler) handle(ctx cosmos.Context, msg MsgSend) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSend", "from", msg.FromAddress, "to", msg.ToAddress, "coins", msg.Amount)
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return nil, errBadVersion
}

func (h SendHandler) handleV1(ctx cosmos.Context, msg MsgSend) (*cosmos.Result, error) {
	haltHeight, err := h.mgr.Keeper().GetMimir(ctx, "HaltTHORChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && ctx.BlockHeight() > haltHeight {
		return nil, fmt.Errorf("mimir has halted THORChain transactions")
	}

	nativeTxFee, err := h.mgr.Keeper().GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = h.mgr.GetConstants().GetInt64Value(constants.NativeTransactionFee)
	}

	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, ErrInternal(err, "fail to get gas fee")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(msg.Amount...)
	if !h.mgr.Keeper().HasCoins(ctx, msg.FromAddress, totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.mgr.Keeper().SendFromAccountToModule(ctx, msg.FromAddress, ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	sdkErr = h.mgr.Keeper().SendCoins(ctx, msg.FromAddress, msg.ToAddress, msg.Amount)
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

// SendAnteHandler called by the ante handler to gate mempool entry
// and also during deliver. Store changes will persist if this function
// succeeds, regardless of the success of the transaction.
func SendAnteHandler(ctx cosmos.Context, v semver.Version, k keeper.Keeper, msg MsgSend) error {
	return nil
}
