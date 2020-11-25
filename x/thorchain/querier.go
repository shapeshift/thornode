package thorchain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/codec"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	q "gitlab.com/thorchain/thornode/x/thorchain/query"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// NewQuerier is the module level router for state queries
func NewQuerier(keeper keeper.Keeper, kbs KeybaseStore) cosmos.Querier {
	return func(ctx cosmos.Context, path []string, req abci.RequestQuery) (res []byte, err error) {
		switch path[0] {
		case q.QueryPool.Key:
			return queryPool(ctx, path[1:], req, keeper)
		case q.QueryPools.Key:
			return queryPools(ctx, req, keeper)
		case q.QueryLiquidityProviders.Key:
			return queryLiquidityProviders(ctx, path[1:], req, keeper)
		case q.QueryTxVoter.Key:
			return queryTxVoters(ctx, path[1:], req, keeper)
		case q.QueryTx.Key:
			return queryTx(ctx, path[1:], req, keeper)
		case q.QueryKeysignArray.Key:
			return queryKeysign(ctx, kbs, path[1:], req, keeper)
		case q.QueryKeysignArrayPubkey.Key:
			return queryKeysign(ctx, kbs, path[1:], req, keeper)
		case q.QueryKeygensPubkey.Key:
			return queryKeygen(ctx, kbs, path[1:], req, keeper)
		case q.QueryQueue.Key:
			return queryQueue(ctx, path[1:], req, keeper)
		case q.QueryHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, keeper)
		case q.QueryChainHeights.Key:
			return queryLastBlockHeights(ctx, path[1:], req, keeper)
		case q.QueryNode.Key:
			return queryNode(ctx, path[1:], req, keeper)
		case q.QueryNodes.Key:
			return queryNodes(ctx, path[1:], req, keeper)
		case q.QueryInboundAddresses.Key:
			return queryInboundAddresses(ctx, path[1:], req, keeper)
		case q.QueryNetwork.Key:
			return queryNetwork(ctx, keeper)
		case q.QueryBalanceModule.Key:
			return queryBalanceModule(ctx, path[1:], keeper)
		case q.QueryVaultsAsgard.Key:
			return queryAsgardVaults(ctx, keeper)
		case q.QueryVaultsYggdrasil.Key:
			return queryYggdrasilVaults(ctx, keeper)
		case q.QueryVault.Key:
			return queryVault(ctx, path[1:], keeper)
		case q.QueryVaultPubkeys.Key:
			return queryVaultsPubkeys(ctx, keeper)
		case q.QueryConstantValues.Key:
			return queryConstantValues(ctx, path[1:], req, keeper)
		case q.QueryVersion.Key:
			return queryVersion(ctx, path[1:], req, keeper)
		case q.QueryMimirValues.Key:
			return queryMimirValues(ctx, path[1:], req, keeper)
		case q.QueryBan.Key:
			return queryBan(ctx, path[1:], req, keeper)
		case q.QueryRagnarok.Key:
			return queryRagnarok(ctx, keeper)
		case q.QueryPendingOutbound.Key:
			return queryPendingOutbound(ctx, keeper)
		case q.QueryTssKeygenMetrics.Key:
			return queryTssKeygenMetric(ctx, path[1:], req, keeper)
		case q.QueryTssMetrics.Key:
			return queryTssMetric(ctx, path[1:], req, keeper)
		default:
			return nil, cosmos.ErrUnknownRequest(
				fmt.Sprintf("unknown thorchain query endpoint: %s", path[0]),
			)
		}
	}
}

func queryRagnarok(ctx cosmos.Context, keeper keeper.Keeper) ([]byte, error) {
	ragnarokInProgress := keeper.RagnarokInProgress(ctx)
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), ragnarokInProgress)
	if err != nil {
		return nil, ErrInternal(err, "fail to marshal response to json")
	}
	return res, nil
}

func queryBalanceModule(ctx cosmos.Context, path []string, keeper keeper.Keeper) ([]byte, error) {
	supplier := keeper.Supply()
	mod := supplier.GetModuleAccount(ctx, AsgardName)

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), mod.GetCoins())
	if err != nil {
		return nil, ErrInternal(err, "fail to marshal response to json")
	}

	return res, nil
}

func queryVault(ctx cosmos.Context, path []string, keeper keeper.Keeper) ([]byte, error) {
	if len(path) < 2 {
		return nil, errors.New("not enough parameters")
	}
	chain, err := common.NewChain(path[0])
	if err != nil {
		return nil, fmt.Errorf("%s is invalid chain,%w", path[0], err)
	}
	addr, err := common.NewAddress(path[1])
	if err != nil {
		return nil, fmt.Errorf("%s is invalid address,%w", path[1], err)
	}
	iter := keeper.GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var v Vault
		if err := keeper.Cdc().UnmarshalBinaryBare(iter.Value(), &v); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}
		vaultAddr, err := v.PubKey.GetAddress(chain)
		if err != nil {
			ctx.Logger().Error("fail to get vault address", "error", err, "chain", chain.String())
			continue
		}
		if vaultAddr.Equals(addr) {

			res, err := codec.MarshalJSONIndent(keeper.Cdc(), v)
			if err != nil {
				ctx.Logger().Error("fail to marshal vaults response to json", "error", err)
				return nil, fmt.Errorf("fail to marshal response to json: %w", err)
			}
			return res, nil
		}
	}
	return nil, errors.New("vault not found")
}

func queryAsgardVaults(ctx cosmos.Context, keeper keeper.Keeper) ([]byte, error) {
	vaults, err := keeper.GetAsgardVaults(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get asgard vaults: %w", err)
	}

	var vaultsWithFunds Vaults
	for _, vault := range vaults {
		if vault.Status == InactiveVault {
			continue
		}
		if !vault.IsAsgard() {
			continue
		}
		if vault.HasFunds() || vault.Status == ActiveVault {
			vaultsWithFunds = append(vaultsWithFunds, vault)
		}
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), vaultsWithFunds)
	if err != nil {
		ctx.Logger().Error("fail to marshal vaults response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}

	return res, nil
}

func getVaultChainAddress(ctx cosmos.Context, vault Vault) []QueryChainAddress {
	var result []QueryChainAddress
	allChains := append(vault.Chains, common.THORChain)
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

func queryYggdrasilVaults(ctx cosmos.Context, keeper keeper.Keeper) ([]byte, error) {
	vaults := make(Vaults, 0)
	iter := keeper.GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := keeper.Cdc().UnmarshalBinaryBare(iter.Value(), &vault); err != nil {
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
		na, err := keeper.GetNodeAccountByPubKey(ctx, vault.PubKey)
		if err != nil {
			ctx.Logger().Error("fail to get node account by pubkey", "error", err)
			continue
		}

		// calculate the total value of this yggdrasil vault
		for _, coin := range vault.Coins {
			if coin.Asset.IsRune() {
				totalValue = totalValue.Add(coin.Amount)
			} else {
				pool, err := keeper.GetPool(ctx, coin.Asset)
				if err != nil {
					ctx.Logger().Error("fail to get pool", "error", err)
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}

		respVaults[i] = QueryYggdrasilVaults{
			Vault:      vault,
			Status:     na.Status,
			Bond:       na.Bond,
			TotalValue: totalValue,
			Addresses:  getVaultChainAddress(ctx, vault),
		}
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), respVaults)
	if err != nil {
		ctx.Logger().Error("fail to marshal vaults response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}

	return res, nil
}

func queryVaultsPubkeys(ctx cosmos.Context, keeper keeper.Keeper) ([]byte, error) {
	var resp struct {
		Asgard    common.PubKeys `json:"asgard"`
		Yggdrasil common.PubKeys `json:"yggdrasil"`
	}
	resp.Asgard = make(common.PubKeys, 0)
	resp.Yggdrasil = make(common.PubKeys, 0)
	iter := keeper.GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := keeper.Cdc().UnmarshalBinaryBare(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
		}
		if vault.IsYggdrasil() {
			na, err := keeper.GetNodeAccountByPubKey(ctx, vault.PubKey)
			if err != nil {
				ctx.Logger().Error("fail to unmarshal vault", "error", err)
				return nil, fmt.Errorf("fail to unmarshal vault: %w", err)
			}
			if !na.Bond.IsZero() {
				resp.Yggdrasil = append(resp.Yggdrasil, vault.PubKey)
			}
		} else if vault.IsAsgard() {
			if vault.Status == ActiveVault || vault.Status == RetiringVault {
				resp.Asgard = append(resp.Asgard, vault.PubKey)
			}
		}
	}
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), resp)
	if err != nil {
		ctx.Logger().Error("fail to marshal pubkeys response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryNetwork(ctx cosmos.Context, keeper keeper.Keeper) ([]byte, error) {
	data, err := keeper.GetNetwork(ctx)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return nil, fmt.Errorf("fail to get vault: %w", err)
	}
	type NetworkResp struct {
		BondRewardRune cosmos.Uint `json:"bond_reward_rune"` // The total amount of awarded rune for bonders
		TotalBondUnits cosmos.Uint `json:"total_bond_units"` // Total amount of bond units
		TotalReserve   cosmos.Uint `json:"total_reserve"`
	}
	result := NetworkResp{
		BondRewardRune: data.BondRewardRune,
		TotalBondUnits: data.TotalBondUnits,
		TotalReserve:   keeper.GetRuneBalanceOfModule(ctx, ReserveName),
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), result)
	if err != nil {
		ctx.Logger().Error("fail to marshal network data to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryInboundAddresses(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	haltTrading, err := keeper.GetMimir(ctx, "HaltTrading")
	if err != nil {
		ctx.Logger().Error("fail to get HaltTrading mimir", "error", err)
	}
	// when trading is halt , do not return any pool addresses
	halted := (haltTrading > 0 && haltTrading < common.BlockHeight(ctx) && err == nil) || keeper.RagnarokInProgress(ctx)
	active, err := keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active vaults", "error", err)
		return nil, fmt.Errorf("fail to get active vaults: %w", err)
	}

	// TODO: halted trading should be enabled per chain. This will be used to
	// decom a chain and not accept new trades/liquidity providing

	type address struct {
		Chain   common.Chain   `json:"chain"`
		PubKey  common.PubKey  `json:"pub_key"`
		Address common.Address `json:"address"`
		Halted  bool           `json:"halted"`
	}

	var resp struct {
		Current []address `json:"current"`
	}

	version := keeper.GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(version)
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)

	// select vault that is most secure
	vault := keeper.GetMostSecure(ctx, active, signingTransactionPeriod)

	chains := vault.Chains

	if len(chains) == 0 {
		chains = common.Chains{common.RuneAsset().Chain}
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

		addr := address{
			Chain:   chain,
			PubKey:  vault.PubKey,
			Address: vaultAddress,
			Halted:  halted,
		}

		resp.Current = append(resp.Current, addr)
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), resp)
	if err != nil {
		ctx.Logger().Error("fail to marshal current pool address to json", "error", err)
		return nil, fmt.Errorf("fail to marshal current pool address to json: %w", err)
	}

	return res, nil
}

// queryNode return the Node information related to the request node address
// /thorchain/node/{nodeaddress}
func queryNode(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not provided")
	}
	nodeAddress := path[0]
	addr, err := cosmos.AccAddressFromBech32(nodeAddress)
	if err != nil {
		return nil, cosmos.ErrUnknownRequest("invalid account address")
	}

	nodeAcc, err := keeper.GetNodeAccount(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	slashPts, err := keeper.GetNodeAccountSlashPoints(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get node slash points: %w", err)
	}
	jail, err := keeper.GetNodeAccountJail(ctx, nodeAcc.NodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fail to get node jail: %w", err)
	}

	result := NewQueryNodeAccount(nodeAcc)
	result.SlashPoints = slashPts
	result.Jail = jail
	// CurrentAward is an estimation of reward for node in active status
	// Node in other status should not have current reward
	if nodeAcc.Status == NodeActive {
		network, err := keeper.GetNetwork(ctx)
		if err != nil {
			return nil, fmt.Errorf("fail to get network: %w", err)
		}

		// find number of blocks they were well behaved (ie active - slash points)
		earnedBlocks := nodeAcc.CalcBondUnits(common.BlockHeight(ctx), slashPts)
		result.CurrentAward = network.CalcNodeRewards(earnedBlocks)
	}

	chainHeights, err := keeper.GetLastObserveHeight(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
	}
	for c, h := range chainHeights {
		result.ObserveChains = append(result.ObserveChains, types.QueryChainHeight{
			Chain:  c,
			Height: h,
		})
	}
	preflightCheckResult, err := getNodePreflightResult(ctx, keeper, nodeAcc)
	if err != nil {
		ctx.Logger().Error("fail to get node preflight result", "error", err)
	} else {
		result.PreflightStatus = preflightCheckResult
	}
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), result)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal node account to json: %w", err)
	}

	return res, nil
}

func getNodePreflightResult(ctx cosmos.Context, keeper keeper.Keeper, nodeAcc NodeAccount) (QueryNodeAccountPreflightCheck, error) {
	mgr := NewManagers(keeper)
	if err := mgr.BeginBlock(ctx); err != nil {
		return QueryNodeAccountPreflightCheck{}, fmt.Errorf("fail to build manager: %w", err)
	}
	version := keeper.GetLowestActiveVersion(ctx)
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

// queryNodes return all the nodes that has bond
// /thorchain/nodes
func queryNodes(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	nodeAccounts, err := keeper.ListNodeAccountsWithBond(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}

	network, err := keeper.GetNetwork(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get network: %w", err)
	}

	result := make([]QueryNodeAccount, len(nodeAccounts))
	for i, na := range nodeAccounts {
		slashPts, err := keeper.GetNodeAccountSlashPoints(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node slash points: %w", err)
		}
		// find number of blocks they were well behaved (ie active - slash points)
		earnedBlocks := na.CalcBondUnits(common.BlockHeight(ctx), slashPts)

		result[i] = NewQueryNodeAccount(na)
		result[i].SlashPoints = slashPts
		if na.Status == NodeActive {
			result[i].CurrentAward = network.CalcNodeRewards(earnedBlocks)
		}

		jail, err := keeper.GetNodeAccountJail(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get node jail: %w", err)
		}
		result[i].Jail = jail
		chainHeights, err := keeper.GetLastObserveHeight(ctx, na.NodeAddress)
		if err != nil {
			return nil, fmt.Errorf("fail to get last observe chain height: %w", err)
		}
		for c, h := range chainHeights {
			result[i].ObserveChains = append(result[i].ObserveChains, types.QueryChainHeight{
				Chain:  c,
				Height: h,
			})
		}
		preflightCheckResult, err := getNodePreflightResult(ctx, keeper, na)
		if err != nil {
			ctx.Logger().Error("fail to get node preflight result", "error", err)
		} else {
			result[i].PreflightStatus = preflightCheckResult
		}
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), result)
	if err != nil {
		ctx.Logger().Error("fail to marshal observers to json", "error", err)
		return nil, fmt.Errorf("fail to marshal observers to json: %w", err)
	}

	return res, nil
}

// queryLiquidityProviders
func queryLiquidityProviders(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to get parse asset", "error", err)
		return nil, fmt.Errorf("fail to parse asset: %w", err)
	}
	var lps LiquidityProviders
	iterator := keeper.GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		keeper.Cdc().MustUnmarshalBinaryBare(iterator.Value(), &lp)
		lps = append(lps, lp)
	}
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), lps)
	if err != nil {
		ctx.Logger().Error("fail to marshal liquidity providers to json", "error", err)
		return nil, fmt.Errorf("fail to marshal liquidity providers to json: %w", err)
	}
	return res, nil
}

// nolint: unparam
func queryPool(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("asset not provided")
	}
	asset, err := common.NewAsset(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse asset", "error", err)
		return nil, fmt.Errorf("could not parse asset: %w", err)
	}

	pool, err := keeper.GetPool(ctx, asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return nil, fmt.Errorf("could not get pool: %w", err)
	}
	if pool.IsEmpty() {
		return nil, fmt.Errorf("pool: %s doesn't exist", path[0])
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), pool)
	if err != nil {
		return nil, fmt.Errorf("could not marshal result to JSON: %w", err)
	}
	return res, nil
}

func queryPools(ctx cosmos.Context, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	pools := Pools{}
	iterator := keeper.GetPoolIterator(ctx)
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().UnmarshalBinaryBare(iterator.Value(), &pool); err != nil {
			return nil, fmt.Errorf("fail to unmarshal pool: %w", err)
		}
		// ignore pool if no liquidity provider units
		if pool.PoolUnits.IsZero() {
			continue
		}
		pools = append(pools, pool)
	}
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), pools)
	if err != nil {
		return nil, fmt.Errorf("could not marshal pools result to json: %w", err)
	}
	return res, nil
}

func queryTxVoters(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return nil, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := keeper.GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx voter", "error", err)
		return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	// when tx in voter doesn't exist , double check tx out voter
	if len(voter.Txs) == 0 {
		voter, err = keeper.GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), voter)
	if err != nil {
		ctx.Logger().Error("fail to marshal tx hash to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tx hash to json: %w", err)
	}
	return res, nil
}

func queryTx(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("tx id not provided")
	}
	hash, err := common.NewTxID(path[0])
	if err != nil {
		ctx.Logger().Error("fail to parse tx id", "error", err)
		return nil, fmt.Errorf("fail to parse tx id: %w", err)
	}
	voter, err := keeper.GetObservedTxInVoter(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx voter", "error", err)
		return nil, fmt.Errorf("fail to get observed tx voter: %w", err)
	}
	if len(voter.Txs) == 0 {
		voter, err = keeper.GetObservedTxOutVoter(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("fail to get observed tx out voter: %w", err)
		}
		if len(voter.Txs) == 0 {
			return nil, fmt.Errorf("tx: %s doesn't exist", hash)
		}
	}

	nodeAccounts, err := keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get node accounts: %w", err)
	}
	keysignMetric, err := keeper.GetTssKeysignMetric(ctx, hash)
	if err != nil {
		ctx.Logger().Error("fatil to get keysign metrics", "error", err)
	}
	result := struct {
		ObservedTx     `json:"observed_tx"`
		KeysignMetrics types.TssKeysignMetric `json:"keysign_metric"`
	}{
		ObservedTx:     voter.GetTx(nodeAccounts),
		KeysignMetrics: *keysignMetric,
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), result)
	if err != nil {
		ctx.Logger().Error("fail to marshal tx hash to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tx hash to json: %w", err)
	}
	return res, nil
}

func queryKeygen(ctx cosmos.Context, kbs KeybaseStore, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
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

	keygenBlock, err := keeper.GetKeygenBlock(ctx, height)
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
			if keygen.Members.Contains(pk) {
				newKeygenBlock.Keygens = append(newKeygenBlock.Keygens, keygen)
			}
		}
		keygenBlock = newKeygenBlock
	}

	buf, err := keeper.Cdc().MarshalBinaryBare(keygenBlock)
	if err != nil {
		ctx.Logger().Error("fail to marshal keygen block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign(kbs.SignerName, kbs.SignerPasswd, buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keygen", "error", err)
		return nil, fmt.Errorf("fail to sign keygen: %w", err)
	}

	query := QueryKeygenBlock{
		KeygenBlock: keygenBlock,
		Signature:   base64.StdEncoding.EncodeToString(sig),
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), query)
	if err != nil {
		ctx.Logger().Error("fail to marshal keygen block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}
	return res, nil
}

func queryKeysign(ctx cosmos.Context, kbs KeybaseStore, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
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

	txs, err := keeper.GetTxOut(ctx, height)
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

	buf, err := keeper.Cdc().MarshalBinaryBare(txs)
	if err != nil {
		ctx.Logger().Error("fail to marshal keysign block to json", "error", err)
		return nil, fmt.Errorf("fail to marshal keysign block to json: %w", err)
	}
	sig, _, err := kbs.Keybase.Sign(kbs.SignerName, kbs.SignerPasswd, buf)
	if err != nil {
		ctx.Logger().Error("fail to sign keysign", "error", err)
		return nil, fmt.Errorf("fail to sign keysign: %w", err)
	}

	query := QueryKeysign{
		Keysign:   *txs,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), query)
	if err != nil {
		ctx.Logger().Error("fail to marshal tx hash to json", "error", err)
		return nil, fmt.Errorf("fail to marshal tx hash to json: %w", err)
	}
	return res, nil
}

// queryOutQueue - iterates over txout, counting how many transactions are waiting to be sent
func queryQueue(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	version := keeper.GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(version)
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := common.BlockHeight(ctx) - signingTransactionPeriod
	query := QueryQueue{}

	iterator := keeper.GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := keeper.Cdc().UnmarshalBinaryBare(iterator.Value(), &msg); err != nil {
			continue
		}
		query.Swap++
	}

	for height := startHeight; height <= common.BlockHeight(ctx); height++ {
		txs, err := keeper.GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				query.Outbound++
			}
		}
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), query)
	if err != nil {
		ctx.Logger().Error("fail to marshal out queue to json", "error", err)
		return nil, fmt.Errorf("fail to marshal out queue to json: %w", err)
	}
	return res, nil
}

func queryLastBlockHeights(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
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
		asgards, err := keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil {
			return nil, fmt.Errorf("fail to get active asgard: %w", err)
		}
		for _, vault := range asgards {
			chains = vault.Chains.Distinct()
			break
		}
	}
	var result []QueryResLastBlockHeights
	for _, c := range chains {
		if c == common.THORChain {
			continue
		}
		chainHeight, err := keeper.GetLastChainHeight(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("fail to get last chain height: %w", err)
		}

		signed, err := keeper.GetLastSignedHeight(ctx)
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

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), result)
	if err != nil {
		ctx.Logger().Error("fail to marshal query response to json", "error", err)
		return nil, fmt.Errorf("fail to marshal response to json: %w", err)
	}
	return res, nil
}

func queryConstantValues(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	ver := keeper.GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(ver)
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), constAccessor)
	if err != nil {
		ctx.Logger().Error("fail to marshal constant values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal constant values to json: %w", err)
	}
	return res, nil
}

func queryVersion(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	ver := QueryVersion{
		Current: keeper.GetLowestActiveVersion(ctx),
		Next:    keeper.GetMinJoinVersion(ctx),
	}
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), ver)
	if err != nil {
		ctx.Logger().Error("fail to marshal version to json", "error", err)
		return nil, fmt.Errorf("fail to marshal version to json: %w", err)
	}
	return res, nil
}

func queryMimirValues(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	values := make(map[string]int64, 0)
	iter := keeper.GetMimirIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var value int64
		if err := keeper.Cdc().UnmarshalBinaryBare(iter.Value(), &value); err != nil {
			ctx.Logger().Error("fail to unmarshal mimir attribute", "error", err)
			return nil, fmt.Errorf("fail to unmarshal mimir attribute:  %w", err)
		}
		values[string(iter.Key())] = value
	}
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), values)
	if err != nil {
		ctx.Logger().Error("fail to marshal mimir values to json", "error", err)
		return nil, fmt.Errorf("fail to marshal mimir values to json: %w", err)
	}
	return res, nil
}

func queryBan(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("node address not available")
	}
	addr, err := cosmos.AccAddressFromBech32(path[0])
	if err != nil {
		ctx.Logger().Error("invalid node address", "error", err)
		return nil, fmt.Errorf("invalid node address: %w", err)
	}

	ban, err := keeper.GetBanVoter(ctx, addr)
	if err != nil {
		ctx.Logger().Error("fail to get ban voter", "error", err)
		return nil, fmt.Errorf("fail to get ban voter: %w", err)
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), ban)
	if err != nil {
		ctx.Logger().Error("fail to marshal ban voter to json", "error", err)
		return nil, fmt.Errorf("fail to ban voter to json: %w", err)
	}
	return res, nil
}

func queryPendingOutbound(ctx cosmos.Context, keeper keeper.Keeper) ([]byte, error) {
	version := keeper.GetLowestActiveVersion(ctx)
	constAccessor := constants.GetConstantValues(version)
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	startHeight := common.BlockHeight(ctx) - signingTransactionPeriod
	var result []TxOutItem
	for height := startHeight; height <= common.BlockHeight(ctx); height++ {
		txs, err := keeper.GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			return nil, fmt.Errorf("fail to get tx out array from key value store: %w", err)
		}
		for _, tx := range txs.TxArray {
			if tx.OutHash.IsEmpty() {
				result = append(result, *tx)
			}
		}
	}

	res, err := codec.MarshalJSONIndent(keeper.Cdc(), result)
	if err != nil {
		ctx.Logger().Error("fail to marshal pending outbound tx to json", "error", err)
		return nil, fmt.Errorf("fail to marshal pending outbound tx to json: %w", err)
	}
	return res, nil
}

func queryTssKeygenMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
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
		m, err := keeper.GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		result = append(result, m)
	}
	res, err := codec.MarshalJSONIndent(keeper.Cdc(), result)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal keygen metrics to json: %w", err)
	}
	return res, nil
}

func queryTssMetric(ctx cosmos.Context, path []string, req abci.RequestQuery, keeper keeper.Keeper) ([]byte, error) {
	var pubKeys common.PubKeys
	// get all active asgard
	vaults, err := keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return nil, fmt.Errorf("fail to get active asgards:%w", err)
	}
	for _, v := range vaults {
		pubKeys = append(pubKeys, v.PubKey)
	}
	var keygenMetrics []*types.TssKeygenMetric
	for _, pkey := range pubKeys {
		m, err := keeper.GetTssKeygenMetric(ctx, pkey)
		if err != nil {
			return nil, fmt.Errorf("fail to get tss keygen metric for pubkey(%s):%w", pkey, err)
		}
		if len(m.NodeTssTimes) == 0 {
			continue
		}
		keygenMetrics = append(keygenMetrics, m)
	}
	keysignMetric, err := keeper.GetLatestTssKeysignMetric(ctx)
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
	return codec.MarshalJSONIndent(keeper.Cdc(), m)
}
