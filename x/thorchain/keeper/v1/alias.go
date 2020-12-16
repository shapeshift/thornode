package keeperv1

import (
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

const (
	ModuleName  = types.ModuleName
	ReserveName = types.ReserveName
	AsgardName  = types.AsgardName
	BondName    = types.BondName
	StoreKey    = types.StoreKey

	// Vaults
	AsgardVault    = types.AsgardVault
	YggdrasilVault = types.YggdrasilVault
	ActiveVault    = types.ActiveVault
	InactiveVault  = types.InactiveVault

	// Node status
	NodeActive  = types.Active
	NodeStandby = types.Standby
	NodeUnknown = types.Unknown

	// Bond type
	AsgardKeygen = types.AsgardKeygen
)

var (
	NewPool                    = types.NewPool
	NewJail                    = types.NewJail
	NewTxMarker                = types.NewTxMarker
	NewNetwork                 = types.NewNetwork
	NewObservedTx              = types.NewObservedTx
	NewTssVoter                = types.NewTssVoter
	NewBanVoter                = types.NewBanVoter
	NewErrataTxVoter           = types.NewErrataTxVoter
	NewObservedTxVoter         = types.NewObservedTxVoter
	NewKeygen                  = types.NewKeygen
	NewKeygenBlock             = types.NewKeygenBlock
	NewTxOut                   = types.NewTxOut
	HasSuperMajority           = types.HasSuperMajority
	RegisterCodec              = types.RegisterCodec
	NewNodeAccount             = types.NewNodeAccount
	NewVault                   = types.NewVault
	NewReserveContributor      = types.NewReserveContributor
	GetRandomTx                = types.GetRandomTx
	GetRandomNodeAccount       = types.GetRandomNodeAccount
	GetRandomBNBAddress        = types.GetRandomBNBAddress
	GetRandomBTCAddress        = types.GetRandomBTCAddress
	GetRandomRUNEAddress       = types.GetRandomRUNEAddress
	GetRandomTxHash            = types.GetRandomTxHash
	GetRandomBech32Addr        = types.GetRandomBech32Addr
	GetRandomPubKey            = types.GetRandomPubKey
	GetRandomPubKeySet         = types.GetRandomPubKeySet
	NewObservedNetworkFeeVoter = types.NewObservedNetworkFeeVoter
	NewNetworkFee              = types.NewNetworkFee
	NewTssKeysignFailVoter     = types.NewTssKeysignFailVoter
	SetupConfigForTest         = types.SetupConfigForTest
	NewChainContract           = types.NewChainContract
)

type (
	MsgSwap                  = types.MsgSwap
	Pool                     = types.Pool
	Pools                    = types.Pools
	LiquidityProvider        = types.LiquidityProvider
	ObservedTxs              = types.ObservedTxs
	ObservedTxVoter          = types.ObservedTxVoter
	BanVoter                 = types.BanVoter
	ErrataTxVoter            = types.ErrataTxVoter
	TssVoter                 = types.TssVoter
	TssKeysignFailVoter      = types.TssKeysignFailVoter
	TxOutItem                = types.TxOutItem
	TxOut                    = types.TxOut
	KeygenBlock              = types.KeygenBlock
	ReserveContributors      = types.ReserveContributors
	Vault                    = types.Vault
	Vaults                   = types.Vaults
	Jail                     = types.Jail
	NodeAccount              = types.NodeAccount
	NodeAccounts             = types.NodeAccounts
	NodeStatus               = types.NodeStatus
	Network                  = types.Network
	VaultStatus              = types.VaultStatus
	TxMarker                 = types.TxMarker
	TxMarkers                = types.TxMarkers
	NetworkFee               = types.NetworkFee
	ObservedNetworkFeeVoter  = types.ObservedNetworkFeeVoter
	RagnarokWithdrawPosition = types.RagnarokWithdrawPosition
	TssKeygenMetric          = types.TssKeygenMetric
	TssKeysignMetric         = types.TssKeysignMetric
	ChainContract            = types.ChainContract
)
