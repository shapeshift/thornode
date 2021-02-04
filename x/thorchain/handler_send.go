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
	keeper keeper.Keeper
	mgr    Manager
}

// NewSendHandler create a new instance of SendHandler
func NewSendHandler(keeper keeper.Keeper, mgr Manager) SendHandler {
	return SendHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run the main entry point to process MsgSend
func (h SendHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgSend)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg, version); err != nil {
		ctx.Logger().Error("MsgSend failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg, version, constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to process MsgSend", "error", err)
	}
	return result, err
}

func (h SendHandler) validate(ctx cosmos.Context, msg MsgSend, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errInvalidVersion
}

func (h SendHandler) validateV1(ctx cosmos.Context, msg MsgSend) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// check if we're sending to asgard, bond modules. If we are, forward to the native tx handler
	if msg.ToAddress.Equals(h.keeper.GetModuleAccAddress(AsgardName)) || msg.ToAddress.Equals(h.keeper.GetModuleAccAddress(BondName)) {
		return errors.New("cannot use MsgSend for Asgard or Bond transactions, use MsgDeposit instead")
	}

	return nil
}

func (h SendHandler) handle(ctx cosmos.Context, msg MsgSend, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgSend", "from", msg.FromAddress, "to", msg.ToAddress, "coins", msg.Amount)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	return nil, errBadVersion
}

func (h SendHandler) handleV1(ctx cosmos.Context, msg MsgSend, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	haltHeight, err := h.keeper.GetMimir(ctx, "HaltTHORChain")
	if err != nil {
		return nil, fmt.Errorf("failed to get mimir setting: %w", err)
	}
	if haltHeight > 0 && common.BlockHeight(ctx) > haltHeight {
		return nil, fmt.Errorf("mimir has halted THORChain transactions")
	}

	nativeTxFee, err := h.keeper.GetMimir(ctx, constants.NativeTransactionFee.String())
	if err != nil || nativeTxFee < 0 {
		nativeTxFee = constAccessor.GetInt64Value(constants.NativeTransactionFee)
	}

	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(nativeTxFee)))
	gasFee, err := gas.Native()
	if err != nil {
		return nil, ErrInternal(err, "fail to get gas fee")
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(msg.Amount...)
	if !h.keeper.HasCoins(ctx, msg.FromAddress, totalCoins) {
		return nil, cosmos.ErrInsufficientCoins(err, "insufficient funds")
	}

	// send gas to reserve
	sdkErr := h.keeper.SendFromAccountToModule(ctx, msg.FromAddress, ReserveName, common.NewCoins(gas))
	if sdkErr != nil {
		return nil, fmt.Errorf("unable to send gas to reserve: %w", sdkErr)
	}

	sdkErr = h.keeper.SendCoins(ctx, msg.FromAddress, msg.ToAddress, msg.Amount)
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
