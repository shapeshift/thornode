package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// TssHandler handle MsgTssPool
type TssHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewTssHandler create a new handler to process MsgTssPool
func NewTssHandler(keeper keeper.Keeper, mgr Manager) TssHandler {
	return TssHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run is the main entry for TssHandler
func (h TssHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgTssPool)
	if !ok {
		return nil, errInvalidMessage
	}
	err := h.validate(ctx, *msg, version)
	if err != nil {
		ctx.Logger().Error("msg_tss_pool failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg, version, constAccessor)
	if err != nil {
		ctx.Logger().Error("failed to process MsgTssPool", "error", err)
		return nil, err
	}
	return result, err
}

func (h TssHandler) validate(ctx cosmos.Context, msg MsgTssPool, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h TssHandler) validateV1(ctx cosmos.Context, msg MsgTssPool) error {
	return h.validateCurrent(ctx, msg)
}

func (h TssHandler) validateCurrent(ctx cosmos.Context, msg MsgTssPool) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	keygenBlock, err := h.keeper.GetKeygenBlock(ctx, msg.Height)
	if err != nil {
		return fmt.Errorf("fail to get keygen block from data store: %w", err)
	}

	for _, keygen := range keygenBlock.Keygens {
		for _, member := range keygen.GetMembers() {
			addr, err := member.GetThorAddress()
			if err == nil && addr.Equals(msg.Signer) {
				return nil
			}
		}
	}

	return cosmos.ErrUnauthorized("not authorized")
}

func (h TssHandler) handle(ctx cosmos.Context, msg MsgTssPool, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info("handleMsgTssPool request", "ID:", msg.ID)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version, constAccessor)
	}
	return nil, errBadVersion
}

func (h TssHandler) handleV1(ctx cosmos.Context, msg MsgTssPool, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	return h.handleCurrent(ctx, msg, version, constAccessor)
}

func (h TssHandler) handleCurrent(ctx cosmos.Context, msg MsgTssPool, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	ctx.Logger().Info(fmt.Sprintf("current version: %s", version.String()))
	if !msg.Blame.IsEmpty() {
		ctx.Logger().Error(msg.Blame.String())
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccess() && !msg.PoolPubKey.IsEmpty() {
		metric, err := h.keeper.GetTssKeygenMetric(ctx, msg.PoolPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get keygen metric", "error", err)
		} else {
			ctx.Logger().Info("save keygen metric to db")
			metric.AddNodeTssTime(msg.Signer, msg.KeygenTime)
			h.keeper.SetTssKeygenMetric(ctx, metric)
		}
	}
	voter, err := h.keeper.GetTssVoter(ctx, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to get tss voter: %w", err)
	}

	// when PoolPubKey is empty , which means TssVoter with id(msg.ID) doesn't
	// exist before, this is the first time to create it
	// set the PoolPubKey to the one in msg, there is no reason voter.PubKeys
	// have anything in it either, thus override it with msg.PubKeys as well
	if voter.PoolPubKey.IsEmpty() {
		voter.PoolPubKey = msg.PoolPubKey
		voter.PubKeys = msg.PubKeys
	}
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := constAccessor.GetInt64Value(constants.ObservationDelayFlexibility)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer, msg.Chains) {
		ctx.Logger().Info("signer already signed MsgTssPool", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil

	}
	h.keeper.SetTssVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus() {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = common.BlockHeight(ctx)
		h.keeper.SetTssVoter(ctx, voter)
		h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccess() {
			vaultType := YggdrasilVault
			if msg.KeygenType == AsgardKeygen {
				vaultType = AsgardVault
			}
			chains := voter.ConsensusChains()
			vault := NewVault(common.BlockHeight(ctx), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), h.keeper.GetChainContracts(ctx, chains))
			vault.Membership = voter.PubKeys

			if err := h.keeper.SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
			keygenBlock, err := h.keeper.GetKeygenBlock(ctx, msg.Height)
			if err != nil {
				return nil, fmt.Errorf("fail to get keygen block, err: %w, height: %d", err, msg.Height)
			}
			initVaults, err := h.keeper.GetAsgardVaultsByStatus(ctx, InitVault)
			if err != nil {
				return nil, fmt.Errorf("fail to get init vaults: %w", err)
			}
			if len(initVaults) == len(keygenBlock.Keygens) {
				for _, v := range initVaults {
					v.UpdateStatus(ActiveVault, common.BlockHeight(ctx))
					if err := h.keeper.SetVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to save vault: %w", err)
					}
					if err := h.mgr.VaultMgr().RotateVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to rotate vault: %w", err)
					}
				}
			} else {
				ctx.Logger().Info(fmt.Sprintf("expecting %d vaults, however only got %d so far, let's wait", len(keygenBlock.Keygens), len(initVaults)))
			}

			metric, err := h.keeper.GetTssKeygenMetric(ctx, msg.PoolPubKey)
			if err != nil {
				ctx.Logger().Error("fail to get keygen metric", "error", err)
			} else {
				var total int64
				for _, item := range metric.NodeTssTimes {
					total += item.TssTime
				}
				evt := NewEventTssKeygenMetric(metric.PubKey, metric.GetMedianTime())
				if err := h.mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
					ctx.Logger().Error("fail to emit tss metric event", "error", err)
				}
			}
		} else {
			// if a node fail to join the keygen, thus hold off the network
			// from churning then it will be slashed accordingly
			slashPoints := constAccessor.GetInt64Value(constants.FailKeygenSlashPoints)
			totalSlash := cosmos.ZeroUint()
			for _, node := range msg.Blame.BlameNodes {
				nodePubKey, err := common.NewPubKey(node.Pubkey)
				if err != nil {
					return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
				}

				na, err := h.keeper.GetNodeAccountByPubKey(ctx, nodePubKey)
				if err != nil {
					return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
				}
				if na.Status == NodeActive {
					if err := h.keeper.IncNodeAccountSlashPoints(ctx, na.NodeAddress, slashPoints); err != nil {
						ctx.Logger().Error("fail to inc slash points", "error", err)
					}

					if err := h.mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keygen")); err != nil {
						ctx.Logger().Error("fail to emit slash point event")
					}
				} else {
					// go to jail
					jailTime := constAccessor.GetInt64Value(constants.JailTimeKeygen)
					releaseHeight := common.BlockHeight(ctx) + jailTime
					reason := "failed to perform keygen"
					if err := h.keeper.SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
						ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
					}

					// take out bond from the node account and add it to vault bond reward RUNE
					// thus good behaviour node will get reward
					reserveVault, err := h.keeper.GetNetwork(ctx)
					if err != nil {
						return nil, fmt.Errorf("fail to get reserve vault: %w", err)
					}

					slashBond := reserveVault.CalcNodeRewards(cosmos.NewUint(uint64(slashPoints)))
					if slashBond.GT(na.Bond) {
						slashBond = na.Bond
					}
					ctx.Logger().Info("fail keygen , slash bond", "address", na.NodeAddress, "amount", slashBond.String())
					na.Bond = common.SafeSub(na.Bond, slashBond)
					totalSlash = totalSlash.Add(slashBond)
					coin := common.NewCoin(common.RuneNative, slashBond)
					if !coin.Amount.IsZero() {
						if err := h.keeper.SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
							return nil, fmt.Errorf("fail to transfer funds from bond to reserve: %w", err)
						}
					}
				}
				if err := h.keeper.SetNodeAccount(ctx, na); err != nil {
					return nil, fmt.Errorf("fail to save node account: %w", err)
				}

				tx := common.Tx{}
				tx.ID = common.BlankTxID
				tx.FromAddress = na.BondAddress
				bondEvent := NewEventBond(totalSlash, BondCost, tx)
				if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
					return nil, fmt.Errorf("fail to emit bond event: %w", err)
				}

			}

		}
		return &cosmos.Result{}, nil
	}

	if (voter.BlockHeight + observeFlex) >= common.BlockHeight(ctx) {
		h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}
