package keeperv1

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

var WhitelistedArbs = []string{ // treasury addresses
	"thor1egxvam70a86jafa8gcg3kqfmfax3s0m2g3m754",
	"bc1qq2z2f4gs4nd7t0a9jjp90y9l9zzjtegu4nczha",
	"qz7262r7uufxk89ematxrf6yquk7zfwrjqm97vskzw",
	"0x04c5998ded94f89263370444ce64a99b7dbc9f46",
	"bnb1pa6hpjs7qv0vkd5ks5tqa2xtt2gk5n08yw7v7f",
	"ltc1qaa064vvv4d6stgywnf777j6dl8rd3tt93fp6jx",
}

func (k KVStore) IsTradingHalt(ctx cosmos.Context, msg cosmos.Msg) bool {
	switch m := msg.(type) {
	case *MsgSwap:
		for _, raw := range WhitelistedArbs {
			address, err := common.NewAddress(strings.TrimSpace(raw))
			if err != nil {
				ctx.Logger().Error("failt to parse address for trading halt check", "address", raw, "error", err)
				continue
			}
			if address.Equals(m.Tx.FromAddress) {
				return false
			}
		}
		source := common.EmptyChain
		if len(m.Tx.Coins) > 0 {
			source = m.Tx.Coins[0].Asset.GetLayer1Asset().Chain
		}
		target := m.TargetAsset.GetLayer1Asset().Chain
		return k.IsChainTradingHalted(ctx, source) || k.IsChainTradingHalted(ctx, target) || k.IsGlobalTradingHalted(ctx)
	case *MsgAddLiquidity:
		return k.IsChainTradingHalted(ctx, m.Asset.Chain) || k.IsGlobalTradingHalted(ctx)
	default:
		return k.IsGlobalTradingHalted(ctx)
	}
}

func (k KVStore) IsGlobalTradingHalted(ctx cosmos.Context) bool {
	haltTrading, err := k.GetMimir(ctx, "HaltTrading")
	if err == nil && ((haltTrading > 0 && haltTrading < ctx.BlockHeight()) || k.RagnarokInProgress(ctx)) {
		return true
	}
	return false
}

func (k KVStore) IsChainTradingHalted(ctx cosmos.Context, chain common.Chain) bool {
	mimirKey := fmt.Sprintf("Halt%sTrading", chain)
	haltChainTrading, err := k.GetMimir(ctx, mimirKey)
	if err == nil && (haltChainTrading > 0 && haltChainTrading < ctx.BlockHeight()) {
		ctx.Logger().Info("trading is halt", "chain", chain)
		return true
	}
	// further to check whether the chain is halted
	return k.IsChainHalted(ctx, chain)
}

func (k KVStore) IsChainHalted(ctx cosmos.Context, chain common.Chain) bool {
	haltChain, err := k.GetMimir(ctx, "HaltChainGlobal")
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("global is halt")
		return true
	}

	haltChain, err = k.GetMimir(ctx, "NodePauseChainGlobal")
	if err == nil && haltChain > ctx.BlockHeight() {
		ctx.Logger().Info("node global is halt")
		return true
	}

	haltMimirKey := fmt.Sprintf("Halt%sChain", chain)
	haltChain, err = k.GetMimir(ctx, haltMimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt via admin or double-spend check", "chain", chain)
		return true
	}

	solvencyHaltMimirKey := fmt.Sprintf("SolvencyHalt%sChain", chain)
	haltChain, err = k.GetMimir(ctx, solvencyHaltMimirKey)
	if err == nil && (haltChain > 0 && haltChain < ctx.BlockHeight()) {
		ctx.Logger().Info("chain is halt via solvency check", "chain", chain)
		return true
	}
	return false
}

func (k KVStore) IsLPPaused(ctx cosmos.Context, chain common.Chain) bool {
	// check if global LP is paused
	pauseLPGlobal, err := k.GetMimir(ctx, "PauseLP")
	if err == nil && pauseLPGlobal > 0 && pauseLPGlobal < ctx.BlockHeight() {
		return true
	}

	pauseLP, err := k.GetMimir(ctx, fmt.Sprintf("PauseLP%s", chain))
	if err == nil && pauseLP > 0 && pauseLP < ctx.BlockHeight() {
		ctx.Logger().Info("chain has paused LP actions", "chain", chain)
		return true
	}
	return false
}
