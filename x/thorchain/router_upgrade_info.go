//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package thorchain

var (
	ethUSDTAsset = `ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7`
	// https://etherscan.io/address/0x42A5Ed456650a09Dc10EBc6361A7480fDd61f27B
	ethOldRouter = `0x42A5Ed456650a09Dc10EBc6361A7480fDd61f27B`
	// https://etherscan.io/address/0xC145990E84155416144C532E31f89B840Ca8c2cE
	ethNewRouter = `0xC145990E84155416144C532E31f89B840Ca8c2cE`
	// https://etherscan.io/address/0x3efF38C0e1e5DD6Bd58d3fa79cAecc4Da46C8866
	temporaryUSDTHolder = `0x3efF38C0e1e5DD6Bd58d3fa79cAecc4Da46C8866`
	ethRouterV3         = `0x3624525075b88B24ecc29CE226b0CEc1fFcB6976`
)
