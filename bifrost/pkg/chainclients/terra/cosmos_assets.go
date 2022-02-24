package terra

type Asset struct {
	CosmosDenom     string
	CosmosDecimals  int
	THORChainSymbol string
}

// CosmosAssets maps a Cosmos denom to a THORChain symbol and provides the asset decimals
// CHANGEME: define assets that should be observed by THORChain here. This also acts a whitelist.
var CosmosAssets = []Asset{
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

func GetAssetByCosmosDenom(denom string) (Asset, bool) {
	for _, asset := range CosmosAssets {
		if asset.CosmosDenom == denom {
			return asset, true
		}
	}
	return Asset{}, false
}

func GetAssetByThorchainSymbol(symbol string) (Asset, bool) {
	for _, asset := range CosmosAssets {
		if asset.THORChainSymbol == symbol {
			return asset, true
		}
	}
	return Asset{}, false
}
