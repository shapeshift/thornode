package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// ReserveContributorHandler is handler to process MsgReserveContributor
type ReserveContributorHandler struct {
	keeper                Keeper
	versionedEventManager VersionedEventManager
}

// NewReserveContributorHandler create a new instance of ReserveContributorHandler
func NewReserveContributorHandler(keeper Keeper, versionedEventManager VersionedEventManager) ReserveContributorHandler {
	return ReserveContributorHandler{
		keeper:                keeper,
		versionedEventManager: versionedEventManager,
	}
}

func (h ReserveContributorHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgReserveContributor)
	if !ok {
		return errInvalidMessage.Result()
	}
	if err := h.Validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("MsgReserveContributor failed validation", "error", err)
		return err.Result()
	}
	return h.Handle(ctx, msg, version)
}

func (h ReserveContributorHandler) Validate(ctx cosmos.Context, msg MsgReserveContributor, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.ValidateV1(ctx, msg)
	}
	return errBadVersion
}

func (h ReserveContributorHandler) ValidateV1(ctx cosmos.Context, msg MsgReserveContributor) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized("not authorized")
	}

	return nil
}

func (h ReserveContributorHandler) Handle(ctx cosmos.Context, msg MsgReserveContributor, version semver.Version) cosmos.Result {
	ctx.Logger().Info("handleMsgReserveContributor request")
	if version.GTE(semver.MustParse("0.1.0")) {
		if err := h.HandleV1(ctx, msg, version); err != nil {
			ctx.Logger().Error("fail to process MsgReserveContributor", "error", err)
			return cosmos.ErrInternal("fail to process reserve contributor").Result()
		}
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}
	ctx.Logger().Error(errInvalidVersion.Error())
	return errBadVersion.Result()
}

// HandleV1  process MsgReserveContributor
func (h ReserveContributorHandler) HandleV1(ctx cosmos.Context, msg MsgReserveContributor, version semver.Version) error {
	reses, err := h.keeper.GetReservesContributors(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get reserve contributors", "error", err)
		return err
	}

	reses = reses.Add(msg.Contributor)
	if err := h.keeper.SetReserveContributors(ctx, reses); err != nil {
		ctx.Logger().Error("fail to save reserve contributors", "error", err)
		return err
	}

	vault, err := h.keeper.GetVaultData(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get vault data", "error", err)
		return err
	}

	vault.TotalReserve = vault.TotalReserve.Add(msg.Contributor.Amount)
	if err := h.keeper.SetVaultData(ctx, vault); err != nil {
		ctx.Logger().Error("fail to save vault data", "error", err)
		return err
	}
	eventMgr, err := h.versionedEventManager.GetEventManager(ctx, version)
	if err != nil {
		return errFailGetEventManager
	}
	reserveEvent := NewEventReserve(msg.Contributor, msg.Tx)
	if err := eventMgr.EmitReserveEvent(ctx, h.keeper, reserveEvent); err != nil {
		return fmt.Errorf("fail to emit reserve event: %w", err)
	}
	return nil
}
