package thorchain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	types2 "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	q "gitlab.com/thorchain/thornode/x/thorchain/query"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// NewQuerier is the module level router for state queries
func NewQuerier(mgr *Mgrs, kbs cosmos.KeybaseStore) cosmos.Querier {
	return func(ctx cosmos.Context, path []string, req abci.RequestQuery) (res []byte, err error) {
		defer telemetry.MeasureSince(time.Now(), path[0])
		switch path[0] {
		case q.QueryPool.Key:
			return queryPool(ctx, path[1:], req, mgr)
		case q.QueryPools.Key:
			return queryPools(ctx, req, mgr)
		case q.QueryLiquidityProviders.Key:
			return queryLiquidityProviders(ctx, path[1:], req, mgr)
		case q.QueryLiquidityProvider.Key:
			return queryLiquidityProvider(ctx, path[1:], req, mgr)
		case q.QueryTxVoter.Key:
			return queryTxVoters(ctx, path[1:], req, mgr)
		case q.QueryTx.Key:
			return queryTx(ctx, path[1:], req, mgr)
		case q.QueryKeysignArray.Key:
			return queryKeysign(ctx, kbs, path[1:], req, mgr)
		case q.QueryKeysignArrayPubkey.Key:
			return queryKeysign(ctx, kbs, path[1:], req, mgr)
		case q.QueryKeygensPubkey.Key:
			return queryKeygen(ctx, kbs, path[1:], req, mgr)
		case q.QueryQueue.Key:
			return queryQueue(ctx, path[1:], req, mgr)
		case q.QueryHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, mgr)
		case q.QueryChainHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, mgr)
		case q.QueryNode.Key:
			return queryNode(ctx, path[1:], req, mgr)
		case q.QueryNodes.Key:
			return queryNodes(ctx, path[1:], req, mgr)
		case q.QueryInboundAddresses.Key:
			return queryInboundAddresses(ctx, path[1:], req, mgr)
		case q.QueryNetwork.Key:
			return queryNetwork(ctx, mgr)
		case q.QueryBalanceModule.Key:
			return queryBalanceModule(ctx, path[1:], mgr)
		case q.QueryVaultsAsgard.Key:
			return queryAsgardVaults(ctx, mgr)
		case q.QueryVaultsYggdrasil.Key:
			return queryYggdrasilVaults(ctx, mgr)
		case q.QueryVault.Key:
			return queryVault(ctx, path[1:], mgr)
		case q.QueryVaultPubkeys.Key:
			return queryVaultsPubkeys(ctx, mgr)
		case q.QueryConstantValues.Key:
			return queryConstantValues(ctx, path[1:], req, mgr)
		case q.QueryVersion.Key:
			return queryVersion(ctx, path[1:], req, mgr)
		case q.QueryMimirValues.Key:
			return queryMimirValues(ctx, path[1:], req, mgr)
		case q.QueryMimirWithKey.Key:
			return queryMimirWithKey(ctx, path[1:], req, mgr)
		case q.QueryMimirAdminValues.Key:
			return queryMimirAdminValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodesAllValues.Key:
			return queryMimirNodesAllValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodesValues.Key:
			return queryMimirNodesValues(ctx, path[1:], req, mgr)
		case q.QueryMimirNodeValues.Key:
			return queryMimirNodeValues(ctx, path[1:], req, mgr)
		case q.QueryBan.Key:
			return queryBan(ctx, path[1:], req, mgr)
		case q.QueryRagnarok.Key:
			return queryRagnarok(ctx, mgr)
		case q.QueryPendingOutbound.Key:
			return queryPendingOutbound(ctx, mgr)
		case q.QueryScheduledOutbound.Key:
			return queryScheduledOutbound(ctx, mgr)
		case q.QueryTssKeygenMetrics.Key:
			return queryTssKeygenMetric(ctx, path[1:], req, mgr)
		case q.QueryTssMetrics.Key:
			return queryTssMetric(ctx, path[1:], req, mgr)
		case q.QueryTHORName.Key:
			return queryTHORName(ctx, path[1:], req, mgr)
		default:
			return nil, cosmos.ErrUnknownRequest(
				fmt.Sprintf("unknown thorchain query endpoint: %s", path[0]),
			)
		}
	}
}

func queryRagnarok(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	ragnarokInProgress := mgr.Keeper().RagnarokInProgress(ctx)
	res, err := json.MarshalIndent(ragnarokInProgress, "", "	")
	if err != nil {
		return nil, ErrInternal(err, "fail to marshal response to json")
	}
	return res, nil
}

func queryBalanceModule(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	moduleName := path[0]
	if len(moduleName) == 0 {
		moduleName = AsgardName
	}

	modAddr := mgr.Keeper().GetModuleAccAddress(moduleName)
	bal := mgr.Keeper().GetBalance(ctx, modAddr)
	balance := struct {
		Name    string            `json:"name"`
		Address cosmos.AccAddress `json:"address"`
		Coins   types2.Coins      `json:"coins"`
	}{
		Name:    moduleName,
		Address: modAddr,
		Coins:   bal,
	}
	res, err := json.MarshalIndent(balance, "", "	")
	if err != nil {
		return nil, ErrInternal(err, "fail to marshal response to json")
	}
	return res, nil
}

func queryTHORName(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	name, err := mgr.Keeper().GetTHORName(ctx, path[0])
	if err != nil {
		return nil, ErrInternal(err, "fail to fetch THORName")
	}

	res, err := json.MarshalIndent(name, "", "	")
	if err != nil {
		return nil, ErrInternal(err, "fail to marshal response to json")
	}
	return res, nil
}

func queryVault(ctx cosmos.Context, path []string, mgr *Mgrs) ([]byte, error) {
	if len(path) < 1 {
		return nil, errors.New("not enough parameters")
	}
	pubkey, err := common.NewPubKey(path[0])
	if err != nil {
		return nil, fmt.Errorf("%s is invalid pubkey", path[0])
	}
	v, err := mgr.Keeper().GetVault(ctx, pubkey)
	if err != nil {
		return nil, fmt.Errorf("fail to get vault with pubkey(%s),err:%w", pubkey, err)
	}
	if v.IsEmpty() {
		return nil, errors.New("vault not found")
	}

	resp := types.QueryVaultResp{
		BlockHeight:           v.BlockHeight,
		PubKey:                v.PubKey,
		Coins:                 v.Coins,
		Type:                  v.Type,
		Status:                v.Status,
		StatusSince:           v.StatusSince,
		Membership:            v.Membership,
		Chains:                v.Chains,
		InboundTxCount:        v.InboundTxCount,
		OutboundTxCount:       v.OutboundTxCount,
		PendingTxBlockHeights: v.PendingTxBlockHeights,
		Routers:               v.Routers,
		Addresses:             getVaultChainAddress(ctx, v),
	}
	res, err := json.MarshalIndent(resp, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal vaults response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryAsgardVaults(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgard vaults: %w", err)
	}

	var vaultsWithFunds []types.QueryVaultResp
	for _, vault := range vaults {
		if vault.Status == InactiveVault {
			continue
		}
		if !vault.IsAsgard() {
			continue
		}
		if vault.HasFunds() || vault.Status == ActiveVault {
			vaultsWithFunds = append(vaultsWithFunds, types.QueryVaultResp{
				BlockHeight:           vault.BlockHeight,
				PubKey:                vault.PubKey,
				Coins:                 vault.Coins,
				Type:                  vault.Type,
				Status:                vault.Status,
				StatusSince:           vault.StatusSince,
				Membership:            vault.Membership,
				Chains:                vault.Chains,
				InboundTxCount:        vault.InboundTxCount,
				OutboundTxCount:       vault.OutboundTxCount,
				PendingTxBlockHeights: vault.PendingTxBlockHeights,
				Routers:               vault.Routers,
				Addresses:             getVaultChainAddress(ctx, vault),
			})
		}
	}

	res, err := json.MarshalIndent(vaultsWithFunds, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal vaults response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}

	return res, nil
}

func getVaultChainAddress(ctx cosmos.Context, vault Vault) []QueryChainAddress {
	var result []QueryChainAddress
	allChains := append(vault.GetChains(), common.THORChain)
	for _, c := range allChains.Distinct() {
		addr, err := vault.PubKey.GetAddress(c)
		if err != nil {
			ctx.Logger().Error("fail to get address for %s:%w", c.String(), err)
			continue
		}
		result = append(result,
			QueryChainAddress{
				Chain:   c,
				Address: addr,
			})
	}
	return result
}

func queryYggdrasilVaults(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	vaults := make(Vaults, 0)
	iter := mgr.Keeper().GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal yggdrasil", "error", err)
			return nil, fmt.Errorf("fail to unmarshal yggdrasil: %w", err)
		}
		if vault.IsYggdrasil() && vault.HasFunds() {
			vaults = append(vaults, vault)
		}
	}

	respVaults := make([]QueryYggdrasilVaults, len(vaults))
	for i, vault := range vaults {
		totalValue := cosmos.ZeroUint()

		// find the bond of this node account
		na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account by pubkey", "error", err)
			continue
		}

		// calculate the total value of this yggdrasil vault
		for _, coin := range vault.Coins {
			if coin.Asset.IsRune() {
				totalValue = totalValue.Add(coin.Amount)
			} else {
				pool, err := mgr.Keeper().GetPool(ctx, coin.Asset)
				if err != nil {
					ctx.Logger().Error("fail to get pool", "error", err)
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}

		respVaults[i] = QueryYggdrasilVaults{
			BlockHeight:           vault.BlockHeight,
			PubKey:                vault.PubKey,
			Coins:                 vault.Coins,
			Type:                  vault.Type,
			StatusSince:           vault.StatusSince,
			Membership:            vault.Membership,
			Chains:                vault.Chains,
			InboundTxCount:        vault.InboundTxCount,
			OutboundTxCount:       vault.OutboundTxCount,
			PendingTxBlockHeights: vault.PendingTxBlockHeights,
			Routers:               vault.Routers,
			Status:                na.Status,
			Bond:                  na.Bond,
			TotalValue:            totalValue,
			Addresses:             getVaultChainAddress(ctx, vault),
		}
	}

	res, err := json.MarshalIndent(respVaults, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal vaults response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}

	return res, nil
}

func queryVaultsPubkeys(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	var resp QueryVaultsPubKeys
	resp.Asgard = make([]QueryVaultPubKeyContract, 0)
	resp.Yggdrasil = make([]QueryVaultPubKeyContract, 0)
	iter := mgr.Keeper().GetVaultIterator(ctx)

	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if vault.IsYggdrasil() {
			na, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
			if err != nil {
				ctx.Logger().Error("fail to unmarshal vault", "error", err)
				return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
			}
			if !na.Bond.IsZero() {
				resp.Yggdrasil = append(resp.Yggdrasil, QueryVaultPubKeyContract{
					PubKey:  vault.PubKey,
					Routers: vault.Routers,
				})
			}
		} else if vault.IsAsgard() {
			if vault.Status == ActiveVault || vault.Status == RetiringVault {
				resp.Asgard = append(resp.Asgard, QueryVaultPubKeyContract{
					PubKey:  vault.PubKey,
					Routers: vault.Routers,
				})
			}
		}
	}
	res, err := json.MarshalIndent(resp, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal pubkeys response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryNetwork(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	data, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return nil, fmt.Errorf("fail to get vault: %w", err)
	}
	type NetworkResp struct {
		BondRewardRune  cosmos.Uint `json:"bond_reward_rune"` // The total amount of awarded rune for bonders
		TotalBondUnits  cosmos.Uint `json:"total_bond_units"` // Total amount of bond units
		TotalReserve    cosmos.Uint `json:"total_reserve"`
		BurnedBep2Rune  cosmos.Uint `json:"burned_bep_2_rune"`
		BurnedErc20Rune cosmos.Uint `json:"burned_erc_20_rune"`
	}
	result := NetworkResp{
		BondRewardRune:  data.BondRewardRune,
		TotalBondUnits:  data.TotalBondUnits,
		TotalReserve:    mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName),
		BurnedBep2Rune:  data.BurnedBep2Rune,
		BurnedErc20Rune: data.BurnedErc20Rune,
	}

	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal network data to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryInboundAddresses(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	active, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "error", err)
		return nil, fmt.Errorf("fail to get active vaults: %w", err)
	}

	type address struct {
		Chain   common.Chain   `json:"chain,omitempty"`
		PubKey  common.PubKey  `json:"pub_key,omitempty"`
		Address common.Address `json:"address,omitempty"`
		Router  common.Address `json:"router,omitempty"`
		Halted  bool           `json:"halted"`
		GasRate cosmos.Uint    `json:"gas_rate,omitempty"`
	}
	var resp []address
	version := mgr.Keeper().GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(version)
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)

	// select vault that is most secure
	vault := mgr.Keeper().GetMostSecure(ctx, active, signingTransactionPeriod)

	chains := vault.GetChains()

	if len(chains) == 0 {
		chains = common.Chains{common.RuneAsset().Chain}
	}
	if err := mgr.BeginBlock(ctx); err != nil {
		return nil, fmt.Errorf("fail to build manager: %w", err)
	}
	for _, chain := range chains {
		// tx send to thorchain doesn't need an address , thus here skip it
		if chain == common.THORChain {
			continue
		}
		vaultAddress, err := vault.PubKey.GetAddress(chain)
		if err != nil {
			ctx.Logger().Error("fail to get address for chain", "error", err)
			return nil, fmt.Errorf("fail to get address for chain: %w", err)
		}
		cc := vault.GetContract(chain)
		gasRate := mgr.GasMgr().GetGasRate(ctx, chain)
		// because THORNode is using 1e8, while GWei in ETH is in 1e9, thus the minimum THORNode can represent is 10Gwei
		// here convert the gas rate to Gwei , so api user don't need to convert it , make it easier for people to understand
		if chain.Equals(common.ETHChain) {
			gasRate = gasRate.MulUint64(10)
		}
		addr := address{
			Chain:   chain,
			PubKey:  vault.PubKey,
			Address: vaultAddress,
			Router:  cc.Router,
			Halted:  isGlobalTradingHalted(ctx, mgr) || isChainTradingHalted(ctx, mgr, chain),
			GasRate: gasRate,
		}

		resp = append(resp, addr)
	}

	res, err := json.MarshalIndent(resp, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal current pool address to json", "error", err)
		return nil, fmt.Errorf("fail to marshal current pool address to json: %w", err)
	}

	return res, nil
}

// queryNode return the Node information related to the request node address
// /thorchain/node/{nodeaddress}
func queryNode(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not provided")
	}
	nodeAddress := path[0]
	addr, err := cosmos.AccAddressFromBech32(nodeAddress)
	if err != nil {
		return nil, cosmos.ErrUnknownRequest("invalid account address")
	}

	nodeAcc, err := mgr.Keeper().GetNodeAccount(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node slash points: %w", err)
	}
	jail, err := mgr.Keeper().GetNodeAccountJail(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to get node jail: %w", err)
	}

	bp, err := mgr.Keeper().GetBondProviders(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to get bond providers: %w", err)
	}
	bp.Adjust(nodeAcc.Bond)

	result := NewQueryNodeAccount(nodeAcc)
	result.SlashPoints = slashPts
	result.Jail = jail
	result.BondProviders = bp

	// CurrentAward is an estimation of reward for node in active status
	// Node in other status should not have current reward
	if nodeAcc.Status == NodeActive && !nodeAcc.Bond.IsZero() {
		network, err := mgr.Keeper().GetNetwork(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get network: %w", err)
		}
		vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, fmt.Errorf("fail to get active vaults: %w", err)
		}
		if len(vaults) == 0 {
			return nil, fmt.Errorf("no active vaults")
		}

		// Determine current bond-weighted hard cap
		constAccessor := mgr.GetConstants()

		minBondInRune, err := mgr.Keeper().GetMimir(ctx, constants.MinimumBondInRune.String())
		if minBondInRune < 0 || err != nil {
			minBondInRune = constAccessor.GetInt64Value(constants.MinimumBondInRune)
		}

		validatorMaxRewardRatio, err := mgr.Keeper().GetMimir(ctx, constants.ValidatorMaxRewardRatio.String())
		if validatorMaxRewardRatio < 0 || err != nil {
			validatorMaxRewardRatio = constAccessor.GetInt64Value(constants.ValidatorMaxRewardRatio)
		}

		bondHardCap := cosmos.NewUint(uint64(validatorMaxRewardRatio)).MulUint64(uint64(minBondInRune))
		totalEffectiveBond, err := getTotalEffectiveBond(ctx, mgr, bondHardCap)
		if err != nil {
			return nil, fmt.Errorf("fail to get total effective bond: %w", err)
		}

		lastChurnHeight := vaults[0].BlockHeight

		reward, err := getNodeCurrentRewards(ctx, mgr, nodeAcc, lastChurnHeight, network.BondRewardRune, totalEffectiveBond, bondHardCap)
		if err != nil {
			return nil, fmt.Errorf("fail to get current node rewards: %w", err)
		}

		result.CurrentAward = reward
	}

	chainHeights, err := mgr.Keeper().GetLastObserveHeight(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
	}

	// analyze-ignore(map-iteration)
	for c, h := range chainHeights {
		result.ObserveChains = append(result.ObserveChains, types.QueryChainHeight{
			Chain:  c,
			Height: h,
		})
	}

	preflightCheckResult, err := getNodePreflightResult(ctx, mgr, nodeAcc)
	if err != nil {
		ctx.Logger().Error("fail to get node preflight result", "error", err)
	} else {
		result.PreflightStatus = preflightCheckResult
	}
	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		return nil, fmt.Errorf("fail to marshal node account to json: %w", err)
	}

	return res, nil
}

func getNodePreflightResult(ctx cosmos.Context, mgr *Mgrs, nodeAcc NodeAccount) (QueryNodeAccountPreflightCheck, error) {
	version := mgr.Keeper().GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(version)
	if constAccessor == nil {
		return QueryNodeAccountPreflightCheck{}, fmt.Errorf("constants for version(%s) is not available", version)
	}
	preflightResult := QueryNodeAccountPreflightCheck{}
	status, err := mgr.ValidatorMgr().NodeAccountPreflightCheck(ctx, nodeAcc, constAccessor)
	preflightResult.Status = status
	if err != nil {
		preflightResult.Description = err.Error()
		preflightResult.Code = 1
	} else {
		preflightResult.Description = "OK"
		preflightResult.Code = 0
	}
	return preflightResult, nil
}

// Estimates current rewards for the NodeAccount taking into account bond-weighted rewards and slash points
func getNodeCurrentRewards(ctx cosmos.Context, mgr *Mgrs, nodeAcc NodeAccount, lastChurnHeight int64, totalBondReward, totalEffectiveBond, bondHardCap cosmos.Uint) (cosmos.Uint, error) {
	slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get node slash points: %w", err)
	}

	// Find number of blocks they have been an active node
	totalActiveBlocks := common.BlockHeight(ctx) - lastChurnHeight

	// find number of blocks they were well behaved (ie active - slash points)
	earnedBlocks := totalActiveBlocks - slashPts
	if earnedBlocks < 0 {
		earnedBlocks = 0
	}

	naBond := nodeAcc.Bond
	if naBond.GT(bondHardCap) {
		naBond = bondHardCap
	}

	reward := common.GetShare(naBond, totalEffectiveBond, totalBondReward)
	reward = common.GetShare(cosmos.NewUint(uint64(earnedBlocks)), cosmos.NewUint(uint64(totalActiveBlocks)), reward)
	return reward, nil
}

// Calculates total "effective bond" - the total bond when taking into account the
// Bond-weighted hard-cap
func getTotalEffectiveBond(ctx cosmos.Context, mgr *Mgrs, bondHardCap cosmos.Uint) (cosmos.Uint, error) {
	activeNodes, err := mgr.Keeper().ListValidatorsByStatus(ctx, NodeActive)
	if err != nil {
		return cosmos.ZeroUint(), fmt.Errorf("fail to get active nodes: %w", err)
	}

	totalEffectiveBond := cosmos.ZeroUint()
	for _, item := range activeNodes {
		b := item.Bond
		if item.Bond.GT(bondHardCap) {
			b = bondHardCap
		}

		totalEffectiveBond = totalEffectiveBond.Add(b)
	}

	return totalEffectiveBond, nil
}

// queryNodes return all the nodes that has bond
// /thorchain/nodes
func queryNodes(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	nodeAccounts, err := mgr.Keeper().ListValidatorsWithBond(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	network, err := mgr.Keeper().GetNetwork(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get network: %w", err)
	}

	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get active vaults: %w", err)
	}
	if len(vaults) == 0 {
		return nil, fmt.Errorf("no active vaults")
	}

	// Determine current bond-weighted hard cap
	constAccessor := mgr.GetConstants()

	minBondInRune, err := mgr.Keeper().GetMimir(ctx, constants.MinimumBondInRune.String())
	if minBondInRune < 0 || err != nil {
		minBondInRune = constAccessor.GetInt64Value(constants.MinimumBondInRune)
	}

	validatorMaxRewardRatio, err := mgr.Keeper().GetMimir(ctx, constants.ValidatorMaxRewardRatio.String())
	if validatorMaxRewardRatio < 0 || err != nil {
		validatorMaxRewardRatio = constAccessor.GetInt64Value(constants.ValidatorMaxRewardRatio)
	}

	bondHardCap := cosmos.NewUint(uint64(validatorMaxRewardRatio)).MulUint64(uint64(minBondInRune))
	totalEffectiveBond, err := getTotalEffectiveBond(ctx, mgr, bondHardCap)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate total effective bond: %w", err)
	}

	lastChurnHeight := vaults[0].BlockHeight
	result := make([]QueryNodeAccount, len(nodeAccounts))
	for i, na := range nodeAccounts {
		if na.RequestedToLeave && na.Bond.LTE(cosmos.NewUint(common.One)) {
			// ignore the node , it left and also has very little bond
			continue
		}
		slashPts, err := mgr.Keeper().GetNodeAccountSlashPoints(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node slash points: %w", err)
		}

		result[i] = NewQueryNodeAccount(na)
		result[i].SlashPoints = slashPts
		if na.Status == NodeActive {
			reward, err := getNodeCurrentRewards(ctx, mgr, na, lastChurnHeight, network.BondRewardRune, totalEffectiveBond, bondHardCap)
			if err != nil {
				return nil, fmt.Errorf("fail to get current node rewards: %w", err)
			}

			result[i].CurrentAward = reward
		}

		jail, err := mgr.Keeper().GetNodeAccountJail(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node jail: %w", err)
		}
		result[i].Jail = jail
		chainHeights, err := mgr.Keeper().GetLastObserveHeight(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
		}

		// analyze-ignore(map-iteration)
		for c, h := range chainHeights {
			result[i].ObserveChains = append(result[i].ObserveChains, types.QueryChainHeight{
				Chain:  c,
				Height: h,
			})
		}

		preflightCheckResult, err := getNodePreflightResult(ctx, mgr, na)
		if err != nil {
			ctx.Logger().Error("fail to get node preflight result", "error", err)
		} else {
			result[i].PreflightStatus = preflightCheckResult
		}
		result[i].BondProviders, err = mgr.Keeper().GetBondProviders(ctx, result[i].NodeAddress)
		if err != nil {
			ctx.Logger().Error("fail to get bond providers", "error", err)
		}
		result[i].BondProviders.Adjust(result[i].Bond)
	}

	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal observers to json", "error", err)
		return nil, fmt.Errorf("fail to marshal observers to json: %w", err)
	}

	return res, nil
}

// queryLiquidityProviders
func queryLiquidityProviders(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}
	var lps LiquidityProviders
	iterator := mgr.Keeper().GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &lp)
		lps = append(lps, lp)
	}
	res, err := json.MarshalIndent(lps, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal liquidity providers to json", "error", err)
		return nil, fmt.Errorf("fail to marshal liquidity providers to json: %w", err)
	}
	return res, nil
}

// queryLiquidityProvider
func queryLiquidityProvider(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) < 2 {
		return nil, errors.New("asset/lp not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}
	addr, err := common.NewAddress(path[1])
	if err != nil {
		ctx.Logger().Error("fail to get parse address", "error", err)
		return nil, fmt.Errorf("fail to parse address: %w", err)
	}
	lp, err := mgr.Keeper().GetLiquidityProvider(ctx, asset, addr)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return nil, fmt.Errorf("fail to liquidity provider: %w", err)
	}
	res, err := json.MarshalIndent(lp, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal liquidity provider to json", "error", err)
		return nil, fmt.Errorf("fail to marshal liquidity provider to json: %w", err)
	}
	return res, nil
}

// nolint: unparam
func queryPool(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return nil, fmt.Errorf("could not parse asset: %w", err)
	}

	pool, err := mgr.Keeper().GetPool(ctx, asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("could not get pool: %w", err)
	}
	if pool.IsEmpty() {
		return nil, fmt.Errorf("pool: %s doesn't exist", path[0])
	}
	ver := mgr.Keeper().GetLowestActiveVersion(ctx)
	synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
	pool.CalcUnits(ver, synthSupply)

	p := struct {
		BalanceRune         cosmos.Uint  `json:"balance_rune"`
		BalanceAsset        cosmos.Uint  `json:"balance_asset"`
		Asset               common.Asset `json:"asset"`
		LPUnits             cosmos.Uint  `json:"LP_units"`
		PoolUnits           cosmos.Uint  `json:"pool_units"`
		Status              PoolStatus   `json:"status,omitempty"`
		Decimals            int64        `json:"decimals,omitempty"`
		SynthUnits          cosmos.Uint  `json:"synth_units"`
		SynthSupply         cosmos.Uint  `json:"synth_supply"`
		PendingInboundRune  cosmos.Uint  `json:"pending_inbound_rune"`
		PendingInboundAsset cosmos.Uint  `json:"pending_inbound_asset"`
	}{
		BalanceRune:         pool.BalanceRune,
		BalanceAsset:        pool.BalanceAsset,
		Asset:               pool.Asset,
		LPUnits:             pool.LPUnits,
		PoolUnits:           pool.GetPoolUnits(),
		Status:              pool.Status,
		Decimals:            pool.Decimals,
		SynthUnits:          pool.SynthUnits,
		SynthSupply:         synthSupply,
		PendingInboundRune:  pool.PendingInboundRune,
		PendingInboundAsset: pool.PendingInboundAsset,
	}

	res, err := json.MarshalIndent(p, "", "	")
	if err != nil {
		return nil, fmt.Errorf("could not marshal result to JSON: %w", err)
	}
	return res, nil
}

func queryPools(ctx cosmos.Context, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	type pp struct {
		BalanceRune         cosmos.Uint  `json:"balance_rune"`
		BalanceAsset        cosmos.Uint  `json:"balance_asset"`
		Asset               common.Asset `json:"asset"`
		LPUnits             cosmos.Uint  `json:"LP_units"`
		PoolUnits           cosmos.Uint  `json:"pool_units"`
		Status              PoolStatus   `json:"status,omitempty"`
		Decimals            int64        `json:"decimals,omitempty"`
		SynthUnits          cosmos.Uint  `json:"synth_units"`
		SynthSupply         cosmos.Uint  `json:"synth_supply"`
		PendingInboundRune  cosmos.Uint  `json:"pending_inbound_rune"`
		PendingInboundAsset cosmos.Uint  `json:"pending_inbound_asset"`
	}

	pools := make([]pp, 0)
	iterator := mgr.Keeper().GetPoolIterator(ctx)
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			return nil, fmt.Errorf("fail to unmarshal pool: %w", err)
		}
		// ignore pool if no liquidity provider units
		if pool.LPUnits.IsZero() {
			continue
		}

		ver := mgr.Keeper().GetLowestActiveVersion(ctx)
		synthSupply := mgr.Keeper().GetTotalSupply(ctx, pool.Asset.GetSyntheticAsset())
		pool.CalcUnits(ver, synthSupply)

		p := pp{
			BalanceRune:         pool.BalanceRune,
			BalanceAsset:        pool.BalanceAsset,
			Asset:               pool.Asset,
			LPUnits:             pool.LPUnits,
			PoolUnits:           pool.GetPoolUnits(),
			Status:              pool.Status,
			Decimals:            pool.Decimals,
			SynthUnits:          pool.SynthUnits,
			SynthSupply:         synthSupply,
			PendingInboundRune:  pool.PendingInboundRune,
			PendingInboundAsset: pool.PendingInboundAsset,
		}
		pools = append(pools, p)
	}
	res, err := json.MarshalIndent(pools, "", "	")
	if err != nil {
		return nil, fmt.Errorf("could not marshal pools result to json: %w", err)
	}
	return res, nil
}

func queryTxVoters(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return nil, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx voter", "error", err)
		return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	// when tx in voter doesn't exist , double check tx out voter
	if len(voter.Txs) == 0 {
		voter, err = mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	res, err := json.MarshalIndent(voter, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal tx hash to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tx hash to json: %w", err)
	}
	return res, nil
}

func queryTx(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return nil, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx voter", "error", err)
		return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	if len(voter.Txs) == 0 {
		voter, err = mgr.Keeper().GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}
	keysignMetric, err := mgr.Keeper().GetTssKeysignMetric(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fatil to get keysign metrics", "error", err)
	}
	result := struct {
		ObservedTx     `json:"observed_tx"`
		KeysignMetrics types.TssKeysignMetric `json:"keysign_metric"`
	}{
		KeysignMetrics: *keysignMetric,
	}
	result.ObservedTx = voter.GetTx(nodeAccounts)
	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal tx hash to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tx hash to json: %w", err)
	}
	return res, nil
}

func queryKeygen(ctx cosmos.Context, kbs cosmos.KeybaseStore, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("block height not provided")
	}
	var err error
	height, err := strconv.ParseInt(path[0], 0, 64)
	if err != nil {
		ctx.Logger().Error("fail to parse block height", "error", err)
		return nil, fmt.Errorf("fail to parse block height: %w", err)
	}

	if height > common.BlockHeight(ctx) {
		return nil, fmt.Errorf("block height not available yet")
	}

	keygenBlock, err := mgr.Keeper().GetKeygenBlock(ctx, height)
	if err != nil {
		ctx.Logger().Error("fail to get keygen block", "error", err)
		return nil, fmt.Errorf("fail to get keygen block: %w", err)
	}

	if len(path) > 1 {
		pk, err := common.NewPubKey(path[1])
		if err != nil {
			ctx.Logger().Error("fail to parse pubkey", "error", err)
			return nil, fmt.Errorf("fail to parse pubkey: %w", err)
		}
		// only return those keygen contains the request pub key
		newKeygenBlock := NewKeygenBlock(keygenBlock.Height)
		for _, keygen := range keygenBlock.Keygens {
			if keygen.GetMembers().Contains(pk) {
				newKeygenBlock.Keygens = append(newKeygenBlock.Keygens, keygen)
			}
		}
		keygenBlock = newKeygenBlock
	}

	buf, err := json.Marshal(keygenBlock)
	if err != nil {
		ctx.Logger().Error("fail to marshal keygen block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign("thorchain", buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keygen", "error", err)
		return nil, fmt.Errorf("fail to sign keygen: %w", err)
	}

	query := QueryKeygenBlock{
		KeygenBlock: keygenBlock,
		Signature:   base64.StdEncoding.EncodeToString(sig),
	}

	res, err := json.MarshalIndent(query, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal keygen block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}
	return res, nil
}

func queryKeysign(ctx cosmos.Context, kbs cosmos.KeybaseStore, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("block height not provided")
	}
	var err error
	height, err := strconv.ParseInt(path[0], 0, 64)
	if err != nil {
		ctx.Logger().Error("fail to parse block height", "error", err)
		return nil, fmt.Errorf("fail to parse block height: %w", err)
	}

	if height > common.BlockHeight(ctx) {
		return nil, fmt.Errorf("block height not available yet")
	}

	pk := common.EmptyPubKey
	if len(path) > 1 {
		pk, err = common.NewPubKey(path[1])
		if err != nil {
			ctx.Logger().Error("fail to parse pubkey", "error", err)
			return nil, fmt.Errorf("fail to parse pubkey: %w", err)
		}
	}

	txs, err := mgr.Keeper().GetTxOut(ctx, height)
	if err != nil {
		ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
		return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
	}

	if !pk.IsEmpty() {
		newTxs := &TxOut{
			Height: txs.Height,
		}
		for _, tx := range txs.TxArray {
			if pk.Equals(tx.VaultPubKey) {
				newTxs.TxArray = append(newTxs.TxArray, tx)
			}
		}
		txs = newTxs
	}

	buf, err := json.Marshal(txs)
	if err != nil {
		ctx.Logger().Error("fail to marshal keysign block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keysign block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign("thorchain", buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keysign", "error", err)
		return nil, fmt.Errorf("fail to sign keysign: %w", err)
	}
	query := QueryKeysign{
		Keysign:   *txs,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}

	res, err := json.MarshalIndent(query, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal tx hash to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tx hash to json: %w", err)
	}
	return res, nil
}

// queryOutQueue - iterates over txout, counting how many transactions are waiting to be sent
func queryQueue(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	version := mgr.Keeper().GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(version)
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := common.BlockHeight(ctx) - signingTransactionPeriod
	query := QueryQueue{
		ScheduledOutboundValue: cosmos.ZeroUint(),
	}

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			continue
		}
		query.Swap++
	}

	for height := startHeight; height <= common.BlockHeight(ctx); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				memo, _ := ParseMemoWithTHORNames(ctx, mgr.Keeper(), tx.Memo)
				if memo.IsInternal() {
					query.Internal++
				} else if memo.IsOutbound() {
					query.Outbound++
				}
			}
		}
	}

	// sum outbound value
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset < 0 || err != nil {
		maxTxOutOffset = constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}
	txOutDelayMax, err := mgr.Keeper().GetMimir(ctx, constants.TxOutDelayMax.String())
	if txOutDelayMax <= 0 || err != nil {
		txOutDelayMax = constAccessor.GetInt64Value(constants.TxOutDelayMax)
	}

	for height := common.BlockHeight(ctx) + 1; height <= common.BlockHeight(ctx)+txOutDelayMax; height++ {
		value, err := mgr.Keeper().GetTxOutValue(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > common.BlockHeight(ctx)+maxTxOutOffset && value.IsZero() {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		query.ScheduledOutboundValue = query.ScheduledOutboundValue.Add(value)
	}

	res, err := json.MarshalIndent(query, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal out queue to json", "error", err)
		return nil, fmt.Errorf("fail to marshal out queue to json: %w", err)
	}
	return res, nil
}

func queryLastBlockHeights(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var chains common.Chains
	if len(path) > 0 && len(path[0]) > 0 {
		var err error
		chain, err := common.NewChain(path[0])
		if err != nil {
			ctx.Logger().Error("fail to parse chain", "error", err, "chain", path[0])
			return nil, fmt.Errorf("fail to retrieve chain: %w", err)
		}
		chains = append(chains, chain)
	} else {
		asgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, fmt.Errorf("fail to get active asgard: %w", err)
		}
		for _, vault := range asgards {
			chains = vault.GetChains().Distinct()
			break
		}
	}
	var result []QueryResLastBlockHeights
	for _, c := range chains {
		if c == common.THORChain {
			continue
		}
		chainHeight, err := mgr.Keeper().GetLastChainHeight(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("fail to get last chain height: %w", err)
		}

		signed, err := mgr.Keeper().GetLastSignedHeight(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get last sign height: %w", err)
		}
		result = append(result, QueryResLastBlockHeights{
			Chain:            c,
			LastChainHeight:  chainHeight,
			LastSignedHeight: signed,
			Thorchain:        common.BlockHeight(ctx),
		})
	}

	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal query response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryConstantValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	ver := mgr.Keeper().GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(ver)
	res, err := json.MarshalIndent(constAccessor, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal constant values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal constant values to json: %w", err)
	}
	return res, nil
}

func queryVersion(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	ver := QueryVersion{
		Current: mgr.Keeper().GetLowestActiveVersion(ctx),
		Next:    mgr.Keeper().GetMinJoinVersion(ctx),
	}
	res, err := json.MarshalIndent(ver, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal version to json", "error", err)
		return nil, fmt.Errorf("fail to marshal version to json: %w", err)
	}
	return res, nil
}

func queryMimirWithKey(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 && len(path[0]) == 0 {
		return nil, fmt.Errorf("no mimir key")
	}
	v, err := mgr.Keeper().GetMimir(ctx, path[0])
	if err != nil {
		return nil, fmt.Errorf("fail to get mimir with key:%s, err : %w", path[0], err)
	}

	res, err := json.MarshalIndent(v, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir value to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryMimirValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	values := make(map[string]int64, 0)

	// collect keys
	iter := mgr.Keeper().GetMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		k := strings.TrimLeft(string(iter.Key()), "mimir//")
		values[k] = 0
	}
	iterNode := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iterNode.Close()
	for ; iterNode.Valid(); iterNode.Next() {
		k := strings.TrimLeft(string(iterNode.Key()), "nodemimir//")
		values[k] = 0
	}

	// analyze-ignore(map-iteration)
	for k := range values {
		v, err := mgr.Keeper().GetMimir(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("fail to get mimir, err: %w", err)
		}
		values[k] = v
	}

	res, err := json.MarshalIndent(values, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryMimirAdminValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	values := make(map[string]int64, 0)
	iter := mgr.Keeper().GetMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		value := types.ProtoInt64{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &value); err != nil {
			ctx.Logger().Error("fail to unmarshal mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal mimir value: %w", err)
		}
		k := strings.TrimLeft(string(iter.Key()), "mimir//")
		values[k] = value.GetValue()
	}
	res, err := json.MarshalIndent(values, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryMimirNodesAllValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	mimirs := NodeMimirs{}
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		m := NodeMimirs{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &m); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}
		mimirs.Mimirs = append(mimirs.Mimirs, m.Mimirs...)
	}

	res, err := json.MarshalIndent(mimirs, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryMimirNodesValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	activeNodes, err := mgr.Keeper().ListActiveValidators(ctx)
	if err != nil {
		ctx.Logger().Error("fail to fetch active node accounts", "error", err)
		return nil, fmt.Errorf("fail to fetch active node accounts: %w", err)
	}
	active := activeNodes.GetNodeAddresses()

	values := make(map[string]int64, 0)
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		mimirs := NodeMimirs{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &mimirs); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}
		k := strings.TrimLeft(string(iter.Key()), "nodemimir//")
		if v, ok := mimirs.HasSuperMajority(k, active); ok {
			values[k] = v
		}
	}

	res, err := json.MarshalIndent(values, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryMimirNodeValues(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	acc, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse thor address", "error", err)
		return nil, fmt.Errorf("fail to parse thor address: %w", err)
	}

	values := make(map[string]int64, 0)
	iter := mgr.Keeper().GetNodeMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		mimirs := NodeMimirs{}
		if err := mgr.Keeper().Cdc().Unmarshal(iter.Value(), &mimirs); err != nil {
			ctx.Logger().Error("fail to unmarshal node mimir value", "error", err)
			return nil, fmt.Errorf("fail to unmarshal node mimir value: %w", err)
		}

		k := strings.TrimLeft(string(iter.Key()), "nodemimir//")
		if v, ok := mimirs.Get(k, acc); ok {
			values[k] = v
		}
	}

	res, err := json.MarshalIndent(values, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryBan(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not available")
	}
	addr, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("invalid node address", "error", err)
		return nil, fmt.Errorf("invalid node address: %w", err)
	}

	ban, err := mgr.Keeper().GetBanVoter(ctx, addr)
	if err != nil {
		ctx.Logger().Error("fail to get ban voter", "error", err)
		return nil, fmt.Errorf("fail to get ban voter: %w", err)
	}

	res, err := json.MarshalIndent(ban, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal ban voter to json", "error", err)
		return nil, fmt.Errorf("fail to ban voter to json: %w", err)
	}
	return res, nil
}

func queryScheduledOutbound(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	result := make([]QueryTxOutItem, 0)
	constAccessor := mgr.GetConstants()
	maxTxOutOffset, err := mgr.Keeper().GetMimir(ctx, constants.MaxTxOutOffset.String())
	if maxTxOutOffset < 0 || err != nil {
		maxTxOutOffset = constAccessor.GetInt64Value(constants.MaxTxOutOffset)
	}
	for height := common.BlockHeight(ctx) + 1; height <= common.BlockHeight(ctx)+17280; height++ {
		txOut, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		if height > common.BlockHeight(ctx)+maxTxOutOffset && len(txOut.TxArray) == 0 {
			// we've hit our max offset, and an empty block, we can assume the
			// rest will be empty as well
			break
		}
		for _, toi := range txOut.TxArray {
			result = append(result, NewQueryTxOutItem(toi, height))
		}
	}

	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal scheduled outbound tx to json", "error", err)
		return nil, fmt.Errorf("fail to marshal scheduled outbound tx to json: %w", err)
	}
	return res, nil
}

func queryPendingOutbound(ctx cosmos.Context, mgr *Mgrs) ([]byte, error) {
	version := mgr.Keeper().GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(version)
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := common.BlockHeight(ctx) - signingTransactionPeriod
	var result []TxOutItem
	for height := startHeight; height <= common.BlockHeight(ctx); height++ {
		txs, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				result = append(result, tx)
			}
		}
	}

	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		ctx.Logger().Error("fail to marshal pending outbound tx to json", "error", err)
		return nil, fmt.Errorf("fail to marshal pending outbound tx to json: %w", err)
	}
	return res, nil
}

func queryTssKeygenMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var pubKeys common.PubKeys
	if len(path) > 0 {
		pkey, err := common.NewPubKey(path[0])
		if err != nil {
			return nil, fmt.Errorf("fail to parse pubkey(%s) err:%w", path[0], err)
		}
		pubKeys = append(pubKeys, pkey)
	}
	var result []*types.TssKeygenMetric
	for _, pkey := range pubKeys {
		m, err := mgr.Keeper().GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		result = append(result, m)
	}
	res, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		return nil, fmt.Errorf("fail to marshal keygen metrics to json: %w", err)
	}
	return res, nil
}

func queryTssMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, mgr *Mgrs) ([]byte, error) {
	var pubKeys common.PubKeys
	// get all active asgard
	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get active asgards:%w", err)
	}
	for _, v := range vaults {
		pubKeys = append(pubKeys, v.PubKey)
	}
	var keygenMetrics []*types.TssKeygenMetric
	for _, pkey := range pubKeys {
		m, err := mgr.Keeper().GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		if len(m.NodeTssTimes) == 0 {
			continue
		}
		keygenMetrics = append(keygenMetrics, m)
	}
	keysignMetric, err := mgr.Keeper().GetLatestTssKeysignMetric(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get keysign metric:%w", err)
	}
	m := struct {
		KeygenMetrics []*types.TssKeygenMetric `json:"keygen"`
		KeysignMetric *types.TssKeysignMetric  `json:"keysign"`
	}{
		KeygenMetrics: keygenMetrics,
		KeysignMetric: keysignMetric,
	}
	return json.MarshalIndent(m, "", "	")
}
