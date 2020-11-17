package query

import (
	"fmt"
	"strings"
)

// Query define all the queries
type Query struct {
	Key              string
	EndpointTemplate string
}

// Endpoint return the end point string
func (q Query) Endpoint(args ...string) string {
	count := strings.Count(q.EndpointTemplate, "%s")
	a := args[:count]

	in := make([]interface{}, len(a))
	for i := range in {
		in[i] = a[i]
	}

	return fmt.Sprintf(q.EndpointTemplate, in...)
}

// Path return the path
func (q Query) Path(args ...string) string {
	temp := []string{args[0], q.Key}
	args = append(temp, args[1:]...)
	return fmt.Sprintf("custom/%s", strings.Join(args, "/"))
}

// query endpoints supported by the thorchain Querier
var (
	QueryPool               = Query{Key: "pool", EndpointTemplate: "/%s/pool/{%s}"}
	QueryPools              = Query{Key: "pools", EndpointTemplate: "/%s/pools"}
	QueryLiquidityProviders = Query{Key: "lps", EndpointTemplate: "/%s/pool/{%s}/liquidity_providers"}
	QueryTx                 = Query{Key: "tx", EndpointTemplate: "/%s/tx/{%s}"}
	QueryTxVoter            = Query{Key: "txvoter", EndpointTemplate: "/%s/tx/{%s}/signers"}
	QueryKeysignArray       = Query{Key: "keysign", EndpointTemplate: "/%s/keysign/{%s}"}
	QueryKeysignArrayPubkey = Query{Key: "keysignpubkey", EndpointTemplate: "/%s/keysign/{%s}/{%s}"}
	QueryKeygensPubkey      = Query{Key: "keygenspubkey", EndpointTemplate: "/%s/keygen/{%s}/{%s}"}
	QueryQueue              = Query{Key: "outqueue", EndpointTemplate: "/%s/queue"}
	QueryHeights            = Query{Key: "heights", EndpointTemplate: "/%s/lastblock"}
	QueryChainHeights       = Query{Key: "chainheights", EndpointTemplate: "/%s/lastblock/{%s}"}
	QueryNodes              = Query{Key: "nodes", EndpointTemplate: "/%s/nodes"}
	QueryNode               = Query{Key: "node", EndpointTemplate: "/%s/node/{%s}"}
	QueryInboundAddresses   = Query{Key: "inboundaddresses", EndpointTemplate: "/%s/inbound_addresses"}
	QueryNetworkData        = Query{Key: "networkdata", EndpointTemplate: "/%s/network"}
	QueryBalanceModule      = Query{Key: "balancemodule", EndpointTemplate: "/%s/balance/module/{%s}"}
	QueryVaultsAsgard       = Query{Key: "vaultsasgard", EndpointTemplate: "/%s/vaults/asgard"}
	QueryVaultsYggdrasil    = Query{Key: "vaultsyggdrasil", EndpointTemplate: "/%s/vaults/yggdrasil"}
	QueryVault              = Query{Key: "vault", EndpointTemplate: "/%s/vault/{%s}/{%s}"}
	QueryVaultPubkeys       = Query{Key: "vaultpubkeys", EndpointTemplate: "/%s/vaults/pubkeys"}
	QueryTSSSigners         = Query{Key: "tsssigner", EndpointTemplate: "/%s/vaults/{%s}/signers"}
	QueryConstantValues     = Query{Key: "constants", EndpointTemplate: "/%s/constants"}
	QueryVersion            = Query{Key: "version", EndpointTemplate: "/%s/version"}
	QueryMimirValues        = Query{Key: "mimirs", EndpointTemplate: "/%s/mimir"}
	QueryBan                = Query{Key: "ban", EndpointTemplate: "/%s/ban/{%s}"}
	QueryRagnarok           = Query{Key: "ragnarok", EndpointTemplate: "/%s/ragnarok"}
	QueryPendingOutbound    = Query{Key: "pendingoutbound", EndpointTemplate: "/%s/queue/outbound"}
)

// Queries all queries
var Queries = []Query{
	QueryPool,
	QueryPools,
	QueryLiquidityProviders,
	QueryTxVoter,
	QueryTx,
	QueryKeysignArray,
	QueryKeysignArrayPubkey,
	QueryQueue,
	QueryHeights,
	QueryChainHeights,
	QueryNode,
	QueryNodes,
	QueryInboundAddresses,
	QueryNetworkData,
	QueryBalanceModule,
	QueryVaultsAsgard,
	QueryVaultsYggdrasil,
	QueryVaultPubkeys,
	QueryVault,
	QueryKeygensPubkey,
	QueryTSSSigners,
	QueryConstantValues,
	QueryVersion,
	QueryMimirValues,
	QueryBan,
	QueryRagnarok,
	QueryPendingOutbound,
}
