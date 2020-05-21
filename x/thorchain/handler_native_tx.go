package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type NativeTxHandler struct {
	keeper Keeper
	mgr    Manager
}

func NewNativeTxHandler(keeper Keeper, mgr Manager) NativeTxHandler {
	return NativeTxHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h NativeTxHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgNativeTx)
	if !ok {
		return errInvalidMessage.Result()
	}
	if err := h.validate(ctx, msg, version); err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}
	return h.handle(ctx, msg, version, constAccessor)
}

func (h NativeTxHandler) validate(ctx cosmos.Context, msg MsgNativeTx, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errInvalidVersion
	}
}

func (h NativeTxHandler) validateV1(ctx cosmos.Context, msg MsgNativeTx) error {
	if err := msg.ValidateBasic(); err != nil {
		ctx.Logger().Error(err.Error())
		return err
	}

	return nil
}

func (h NativeTxHandler) handle(ctx cosmos.Context, msg MsgNativeTx, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	ctx.Logger().Info("receive MsgNativeTx", "from", msg.GetSigners()[0], "coins", msg.Coins, "memo", msg.Memo)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion.Result()
	}
}

func (h NativeTxHandler) handleV1(ctx cosmos.Context, msg MsgNativeTx, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	banker := h.keeper.CoinKeeper()
	supplier := h.keeper.Supply()
	// TODO: this shouldn't be tied to swaps, and should be cheaper. But
	// TransactionFee will be fine for now.
	transactionFee := constAccessor.GetInt64Value(constants.TransactionFee)

	gas := common.NewCoin(common.RuneNative, cosmos.NewUint(uint64(transactionFee)))
	gasFee, err := gas.Native()
	if err != nil {
		ctx.Logger().Error("fail to get gas fee", "err", err)
		return cosmos.ErrInternal("fail to get gas fee").Result()
	}

	coins, err := msg.Coins.Native()
	if err != nil {
		ctx.Logger().Error("coins are native to THORChain", "error", err)
		return cosmos.ErrInsufficientCoins("coins are native to THORChain").Result()
	}

	totalCoins := cosmos.NewCoins(gasFee).Add(coins)
	if !banker.HasCoins(ctx, msg.GetSigners()[0], totalCoins) {
		ctx.Logger().Error("insufficient funds", "error", err)
		return cosmos.ErrInsufficientCoins("insufficient funds").Result()
	}

	// send gas to reserve
	sdkErr := supplier.SendCoinsFromAccountToModule(ctx, msg.GetSigners()[0], ReserveName, cosmos.NewCoins(gasFee))
	if sdkErr != nil {
		ctx.Logger().Error("unable to send gas to reserve", "error", sdkErr)
		return sdkErr.Result()
	}

	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	if err != nil {
		ctx.Logger().Error("fail to get tx hash", "err", err)
		return cosmos.ErrInternal("fail to get tx hash").Result()
	}
	from, err := common.NewAddress(msg.GetSigners()[0].String())
	if err != nil {
		ctx.Logger().Error("fail to get from address", "err", err)
		return cosmos.ErrInternal("fail to get from address").Result()
	}
	to, err := common.NewAddress(supplier.GetModuleAddress(AsgardName).String())
	if err != nil {
		ctx.Logger().Error("fail to get to address", "err", err)
		return cosmos.ErrInternal("fail to get to address").Result()
	}

	tx := common.NewTx(txID, from, to, msg.Coins, common.Gas{gas}, msg.Memo)

	handler := NewInternalHandler(h.keeper, h.mgr)

	txOutStore, err := h.versionedTxOutStore.GetTxOutStore(ctx, h.keeper, version)
	if err != nil {
		ctx.Logger().Error("fail to get txout store", "error", err)
		return errBadVersion.Result()
	}

	eventMgr, err := h.versionedEventManager.GetEventManager(ctx, version)
	if err != nil {
		ctx.Logger().Error("fail to get event manager", "error", err)
		return errFailGetEventManager.Result()
	}

	memo, _ := ParseMemo(msg.Memo) // ignore err
	targetModule := AsgardName
	if memo.IsType(TxBond) {
		targetModule = BondName
	}
	// send funds to target module
	sdkErr = supplier.SendCoinsFromAccountToModule(ctx, msg.GetSigners()[0], targetModule, coins)
	if sdkErr != nil {
		return sdkErr.Result()
	}

	// construct msg from memo
	txIn := ObservedTx{Tx: tx}
	m, txErr := processOneTxIn(ctx, h.keeper, txIn, msg.Signer)
	if txErr != nil {
		ctx.Logger().Error("fail to process native inbound tx", "error", txErr.Error(), "tx hash", tx.ID.String())
		if newErr := refundTx(ctx, txIn, txOutStore, h.keeper, constAccessor, txErr.Code(), fmt.Sprint(txErr.Data()), eventMgr); nil != newErr {
			return cosmos.ErrInternal(newErr.Error()).Result()
		}
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}

	result := handler(ctx, m)
	if !result.IsOK() {
		refundMsg, err := getErrMessageFromABCILog(result.Log)
		if err != nil {
			ctx.Logger().Error(err.Error())
		}
		if err := refundTx(ctx, txIn, h.mgr, h.keeper, constAccessor, result.Code, refundMsg); err != nil {
			return cosmos.ErrInternal(err.Error()).Result()
		}
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}
