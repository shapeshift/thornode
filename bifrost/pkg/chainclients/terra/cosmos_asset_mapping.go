package terra

type CosmosAssetMapping struct {
	CosmosDenom     string
	CosmosDecimals  int
	THORChainSymbol string
}

// CosmosAssets maps a Cosmos denom to a THORChain symbol and provides the asset decimals
// CHANGEME: define assets that should be observed by THORChain here. This also acts a whitelist.
var CosmosAssetMappings = []CosmosAssetMapping{
	{
		CosmosDenom:     "uluna",
		CosmosDecimals:  6,
		THORChainSymbol: "LUNA",
	},
	{
		CosmosDenom:     "uusd",
		CosmosDecimals:  6,
		THORChainSymbol: "UST",
	},
}

func GetAssetByCosmosDenom(denom string) (CosmosAssetMapping, bool) {
	for _, asset := range CosmosAssetMappings {
		if asset.CosmosDenom == denom {
			return asset, true
		}
	}
	return CosmosAssetMapping{}, false
}

func GetAssetByThorchainSymbol(symbol string) (CosmosAssetMapping, bool) {
	for _, asset := range CosmosAssetMappings {
		if asset.THORChainSymbol == symbol {
			return asset, true
		}
	}
	return CosmosAssetMapping{}, false
}
