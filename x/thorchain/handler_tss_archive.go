package thorchain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (h TssHandler) handleV92(ctx cosmos.Context, msg MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handler tss", "current version", h.mgr.GetVersion())
	if !msg.Blame.IsEmpty() {
		ctx.Logger().Error(msg.Blame.String())
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccess() && !msg.PoolPubKey.IsEmpty() {
		metric, err := h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get keygen metric", "error", err)
		} else {
			ctx.Logger().Info("save keygen metric to db")
			metric.AddNodeTssTime(msg.Signer, msg.KeygenTime)
			h.mgr.Keeper().SetTssKeygenMetric(ctx, metric)
		}
	}
	voter, err := h.mgr.Keeper().GetTssVoter(ctx, msg.ID)
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
	// voter's pool pubkey is the same as the one in messasge
	if !voter.PoolPubKey.Equals(msg.PoolPubKey) {
		return nil, fmt.Errorf("invalid pool pubkey")
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

	if !voter.Sign(msg.Signer, msg.Chains) {
		ctx.Logger().Info("signer already signed MsgTssPool", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil

	}
	h.mgr.Keeper().SetTssVoter(ctx, voter)

	// doesn't have 2/3 majority consensus yet
	if !voter.HasConsensus() {
		return &cosmos.Result{}, nil
	}

	// when keygen success
	if msg.IsSuccess() {
		h.judgeLateSigner(ctx, msg, voter)
		if !voter.HasCompleteConsensus() {
			return &cosmos.Result{}, nil
		}
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = common.BlockHeight(ctx)
		h.mgr.Keeper().SetTssVoter(ctx, voter)
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccess() {
			vaultType := YggdrasilVault
			if msg.KeygenType == AsgardKeygen {
				vaultType = AsgardVault
			}
			chains := voter.ConsensusChains()
			vault := NewVault(common.BlockHeight(ctx), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), h.mgr.Keeper().GetChainContracts(ctx, chains))
			vault.Membership = voter.PubKeys

			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
			keygenBlock, err := h.mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
			if err != nil {
				return nil, fmt.Errorf("fail to get keygen block, err: %w, height: %d", err, msg.Height)
			}
			initVaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, InitVault)
			if err != nil {
				return nil, fmt.Errorf("fail to get init vaults: %w", err)
			}

			metric, err := h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
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

			if len(initVaults) == len(keygenBlock.Keygens) {
				for _, v := range initVaults {
					v.UpdateStatus(ActiveVault, common.BlockHeight(ctx))
					if err := h.mgr.Keeper().SetVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to save vault: %w", err)
					}
					if err := h.mgr.NetworkMgr().RotateVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to rotate vault: %w", err)
					}
				}
			} else {
				ctx.Logger().Info("not enough keygen yet", "expecting", len(keygenBlock.Keygens), "current", len(initVaults))
			}
		} else {
			// if a node fail to join the keygen, thus hold off the network
			// from churning then it will be slashed accordingly
			slashPoints := h.mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
			for _, node := range msg.Blame.BlameNodes {
				nodePubKey, err := common.NewPubKey(node.Pubkey)
				if err != nil {
					return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
				}

				na, err := h.mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
				if err != nil {
					return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
				}
				if na.Status == NodeActive {
					failedKeygenSlashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
						telemetry.NewLabel("reason", "failed_keygen"),
					}))
					if err := h.mgr.Keeper().IncNodeAccountSlashPoints(failedKeygenSlashCtx, na.NodeAddress, slashPoints); err != nil {
						ctx.Logger().Error("fail to inc slash points", "error", err)
					}

					if err := h.mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keygen")); err != nil {
						ctx.Logger().Error("fail to emit slash point event")
					}
				} else {
					// go to jail
					jailTime := h.mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
					releaseHeight := common.BlockHeight(ctx) + jailTime
					reason := "failed to perform keygen"
					if err := h.mgr.Keeper().SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
						ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
					}

					network, err := h.mgr.Keeper().GetNetwork(ctx)
					if err != nil {
						return nil, fmt.Errorf("fail to get network: %w", err)
					}

					slashBond := network.CalcNodeRewards(cosmos.NewUint(uint64(slashPoints)))
					if slashBond.GT(na.Bond) {
						slashBond = na.Bond
					}
					ctx.Logger().Info("fail keygen , slash bond", "address", na.NodeAddress, "amount", slashBond.String())
					// take out bond from the node account and add it to the Reserve
					// thus good behaviour nodes and liquidity providers will get reward
					na.Bond = common.SafeSub(na.Bond, slashBond)
					coin := common.NewCoin(common.RuneNative, slashBond)
					if !coin.Amount.IsZero() {
						if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
							return nil, fmt.Errorf("fail to transfer funds from bond to reserve: %w", err)
						}
						slashFloat, _ := new(big.Float).SetInt(slashBond.BigInt()).Float32()
						telemetry.IncrCounterWithLabels(
							[]string{"thornode", "bond_slash"},
							slashFloat,
							[]metrics.Label{
								telemetry.NewLabel("address", na.NodeAddress.String()),
								telemetry.NewLabel("reason", "failed_keygen"),
							},
						)
					}

					tx := common.Tx{}
					tx.ID = common.BlankTxID
					tx.FromAddress = na.BondAddress
					bondEvent := NewEventBond(slashBond, BondCost, tx)
					if err := h.mgr.EventMgr().EmitEvent(ctx, bondEvent); err != nil {
						return nil, fmt.Errorf("fail to emit bond event: %w", err)
					}
				}
				if err := h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
					return nil, fmt.Errorf("fail to save node account: %w", err)
				}
			}

		}
		return &cosmos.Result{}, nil
	}

	if (voter.BlockHeight + observeFlex) >= common.BlockHeight(ctx) {
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}

func (h TssHandler) handleV73(ctx cosmos.Context, msg MsgTssPool) (*cosmos.Result, error) {
	ctx.Logger().Info("handler tss", "current version", h.mgr.GetVersion())
	if !msg.Blame.IsEmpty() {
		ctx.Logger().Error(msg.Blame.String())
	}
	// only record TSS metric when keygen is success
	if msg.IsSuccess() && !msg.PoolPubKey.IsEmpty() {
		metric, err := h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
		if err != nil {
			ctx.Logger().Error("fail to get keygen metric", "error", err)
		} else {
			ctx.Logger().Info("save keygen metric to db")
			metric.AddNodeTssTime(msg.Signer, msg.KeygenTime)
			h.mgr.Keeper().SetTssKeygenMetric(ctx, metric)
		}
	}
	voter, err := h.mgr.Keeper().GetTssVoter(ctx, msg.ID)
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
	// voter's pool pubkey is the same as the one in messasge
	if !voter.PoolPubKey.Equals(msg.PoolPubKey) {
		return nil, fmt.Errorf("invalid pool pubkey")
	}
	observeSlashPoints := h.mgr.GetConstants().GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := h.mgr.GetConstants().GetInt64Value(constants.ObservationDelayFlexibility)

	slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
		telemetry.NewLabel("reason", "failed_observe_tss_pool"),
	}))
	h.mgr.Slasher().IncSlashPoints(slashCtx, observeSlashPoints, msg.Signer)

	if !voter.Sign(msg.Signer, msg.Chains) {
		ctx.Logger().Info("signer already signed MsgTssPool", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil

	}
	h.mgr.Keeper().SetTssVoter(ctx, voter)

	// doesn't have 2/3 majority consensus yet
	if !voter.HasConsensus() {
		return &cosmos.Result{}, nil
	}

	// when keygen success
	if msg.IsSuccess() {
		h.judgeLateSigner(ctx, msg, voter)
		if !voter.HasCompleteConsensus() {
			return &cosmos.Result{}, nil
		}
	}

	if voter.BlockHeight == 0 {
		voter.BlockHeight = common.BlockHeight(ctx)
		h.mgr.Keeper().SetTssVoter(ctx, voter)
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, voter.GetSigners()...)
		if msg.IsSuccess() {
			vaultType := YggdrasilVault
			if msg.KeygenType == AsgardKeygen {
				vaultType = AsgardVault
			}
			chains := voter.ConsensusChains()
			vault := NewVault(common.BlockHeight(ctx), InitVault, vaultType, voter.PoolPubKey, chains.Strings(), h.mgr.Keeper().GetChainContracts(ctx, chains))
			vault.Membership = voter.PubKeys

			if err := h.mgr.Keeper().SetVault(ctx, vault); err != nil {
				return nil, fmt.Errorf("fail to save vault: %w", err)
			}
			keygenBlock, err := h.mgr.Keeper().GetKeygenBlock(ctx, msg.Height)
			if err != nil {
				return nil, fmt.Errorf("fail to get keygen block, err: %w, height: %d", err, msg.Height)
			}
			initVaults, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, InitVault)
			if err != nil {
				return nil, fmt.Errorf("fail to get init vaults: %w", err)
			}
			if len(initVaults) == len(keygenBlock.Keygens) {
				for _, v := range initVaults {
					v.UpdateStatus(ActiveVault, common.BlockHeight(ctx))
					if err := h.mgr.Keeper().SetVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to save vault: %w", err)
					}
					if err := h.mgr.NetworkMgr().RotateVault(ctx, v); err != nil {
						return nil, fmt.Errorf("fail to rotate vault: %w", err)
					}
				}
			} else {
				ctx.Logger().Info("not enough keygen yet", "expecting", len(keygenBlock.Keygens), "current", len(initVaults))
			}

			metric, err := h.mgr.Keeper().GetTssKeygenMetric(ctx, msg.PoolPubKey)
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
			slashPoints := h.mgr.GetConstants().GetInt64Value(constants.FailKeygenSlashPoints)
			totalSlash := cosmos.ZeroUint()
			for _, node := range msg.Blame.BlameNodes {
				nodePubKey, err := common.NewPubKey(node.Pubkey)
				if err != nil {
					return nil, ErrInternal(err, fmt.Sprintf("fail to parse pubkey(%s)", node.Pubkey))
				}

				na, err := h.mgr.Keeper().GetNodeAccountByPubKey(ctx, nodePubKey)
				if err != nil {
					return nil, fmt.Errorf("fail to get node from it's pub key: %w", err)
				}
				if na.Status == NodeActive {
					failedKeygenSlashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
						telemetry.NewLabel("reason", "failed_keygen"),
					}))
					if err := h.mgr.Keeper().IncNodeAccountSlashPoints(failedKeygenSlashCtx, na.NodeAddress, slashPoints); err != nil {
						ctx.Logger().Error("fail to inc slash points", "error", err)
					}

					if err := h.mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, "fail keygen")); err != nil {
						ctx.Logger().Error("fail to emit slash point event")
					}
				} else {
					// go to jail
					jailTime := h.mgr.GetConstants().GetInt64Value(constants.JailTimeKeygen)
					releaseHeight := common.BlockHeight(ctx) + jailTime
					reason := "failed to perform keygen"
					if err := h.mgr.Keeper().SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
						ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
					}

					// take out bond from the node account and add it to vault bond reward RUNE
					// thus good behaviour node will get reward
					reserveVault, err := h.mgr.Keeper().GetNetwork(ctx)
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
						if err := h.mgr.Keeper().SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
							return nil, fmt.Errorf("fail to transfer funds from bond to reserve: %w", err)
						}
						slashFloat, _ := new(big.Float).SetInt(slashBond.BigInt()).Float32()
						telemetry.IncrCounterWithLabels(
							[]string{"thornode", "bond_slash"},
							slashFloat,
							[]metrics.Label{
								telemetry.NewLabel("address", na.NodeAddress.String()),
								telemetry.NewLabel("reason", "failed_keygen"),
							},
						)

					}
				}
				if err := h.mgr.Keeper().SetNodeAccount(ctx, na); err != nil {
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
		h.mgr.Slasher().DecSlashPoints(slashCtx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}
