package thorchain

import (
	"errors"
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// NewGenesisState create a new instance of GenesisState
func NewGenesisState() GenesisState {
	return GenesisState{}
}

// ValidateGenesis validate genesis is valid or not
func ValidateGenesis(data GenesisState) error {
	for _, record := range data.Pools {
		if err := record.Valid(); err != nil {
			return err
		}
	}

	for _, voter := range data.ObservedTxInVoters {
		if err := voter.Valid(); err != nil {
			return err
		}
	}

	for _, voter := range data.ObservedTxOutVoters {
		if err := voter.Valid(); err != nil {
			return err
		}
	}

	for _, out := range data.TxOuts {
		if err := out.Valid(); err != nil {
			return err
		}
	}

	for _, ta := range data.NodeAccounts {
		if err := ta.Valid(); err != nil {
			return err
		}
	}

	for _, vault := range data.Vaults {
		if err := vault.Valid(); err != nil {
			return err
		}
	}

	for _, bv := range data.BanVoters {
		if err := bv.Valid(); err != nil {
			return fmt.Errorf("invalid ban voter: %w", err)
		}
	}

	if data.LastSignedHeight < 0 {
		return errors.New("last signed height cannot be negative")
	}
	for c, h := range data.LastChainHeights {
		if h < 0 {
			return fmt.Errorf("invalid chain(%s) height", c)
		}
	}
	for _, r := range data.ReserveContributors {
		if err := r.Valid(); err != nil {
			return fmt.Errorf("invalid reserve contributor:%w", err)
		}
	}

	for _, b := range data.KeygenBlocks {
		for _, kb := range b.Keygens {
			if err := kb.Valid(); err != nil {
				return fmt.Errorf("invalid keygen: %w", err)
			}
		}
	}
	for _, item := range data.MsgSwaps {
		if err := item.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid swap msg: %w", err)
		}
	}
	for _, nf := range data.NetworkFees {
		if err := nf.Valid(); err != nil {
			return fmt.Errorf("invalid network fee: %w", err)
		}
	}

	for _, cc := range data.ChainContracts {
		if cc.IsEmpty() {
			return fmt.Errorf("chain contract cannot be empty")
		}
	}

	return nil
}

// DefaultGenesisState the default values THORNode put in the Genesis
func DefaultGenesisState() GenesisState {
	return GenesisState{
		Pools:                []Pool{},
		NodeAccounts:         NodeAccounts{},
		TxOuts:               make([]TxOut, 0),
		LiquidityProviders:   make(LiquidityProviders, 0),
		Vaults:               make(Vaults, 0),
		ObservedTxInVoters:   make(ObservedTxVoters, 0),
		ObservedTxOutVoters:  make(ObservedTxVoters, 0),
		BanVoters:            make([]BanVoter, 0),
		LastSignedHeight:     0,
		LastChainHeights:     make(map[string]int64),
		ReserveContributors:  ReserveContributors{},
		Network:              NewNetwork(),
		TssVoters:            make([]TssVoter, 0),
		TssKeysignFailVoters: make([]TssKeysignFailVoter, 0),
		KeygenBlocks:         make([]KeygenBlock, 0),
		ErrataTxVoters:       make([]ErrataTxVoter, 0),
		MsgSwaps:             make([]MsgSwap, 0),
		NetworkFees:          make([]NetworkFee, 0),
		NetworkFeeVoters:     make([]ObservedNetworkFeeVoter, 0),
		ChainContracts:       make([]ChainContract, 0),
	}
}

// InitGenesis read the data in GenesisState and apply it to data store
func InitGenesis(ctx cosmos.Context, keeper keeper.Keeper, data GenesisState) []abci.ValidatorUpdate {
	for _, record := range data.Pools {
		if err := keeper.SetPool(ctx, record); err != nil {
			panic(err)
		}
	}

	for _, lp := range data.LiquidityProviders {
		keeper.SetLiquidityProvider(ctx, lp)
	}

	validators := make([]abci.ValidatorUpdate, 0, len(data.NodeAccounts))
	for _, nodeAccount := range data.NodeAccounts {
		if nodeAccount.Status == NodeActive {
			// Only Active node will become validator
			pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, nodeAccount.ValidatorConsPubKey)
			if err != nil {
				ctx.Logger().Error("fail to parse consensus public key", "key", nodeAccount.ValidatorConsPubKey, "error", err)
				panic(err)
			}
			validators = append(validators, abci.Ed25519ValidatorUpdate(pk.Bytes(), 100))
		}

		if err := keeper.SetNodeAccount(ctx, nodeAccount); err != nil {
			// we should panic
			panic(err)
		}
	}

	for _, vault := range data.Vaults {
		if err := keeper.SetVault(ctx, vault); err != nil {
			panic(err)
		}
	}

	for _, voter := range data.ObservedTxInVoters {
		keeper.SetObservedTxInVoter(ctx, voter)
	}

	for _, voter := range data.ObservedTxOutVoters {
		keeper.SetObservedTxOutVoter(ctx, voter)
	}

	for _, bv := range data.BanVoters {
		keeper.SetBanVoter(ctx, bv)
	}

	for _, out := range data.TxOuts {
		if err := keeper.SetTxOut(ctx, &out); err != nil {
			ctx.Logger().Error("fail to save tx out during genesis", "error", err)
			panic(err)
		}
	}

	if data.LastSignedHeight > 0 {
		if err := keeper.SetLastSignedHeight(ctx, data.LastSignedHeight); err != nil {
			panic(err)
		}
	}

	for c, h := range data.LastChainHeights {
		chain, err := common.NewChain(c)
		if err != nil {
			panic(err)
		}
		if err := keeper.SetLastChainHeight(ctx, chain, h); err != nil {
			panic(err)
		}
	}
	if len(data.ReserveContributors) > 0 {
		if err := keeper.SetReserveContributors(ctx, data.ReserveContributors); err != nil {
			// panic(err)
		}
	}
	if err := keeper.SetNetwork(ctx, data.Network); err != nil {
		panic(err)
	}

	for _, tv := range data.TssVoters {
		if tv.IsEmpty() {
			continue
		}
		keeper.SetTssVoter(ctx, tv)
	}
	for _, item := range data.TssKeysignFailVoters {
		if item.Empty() {
			continue
		}
		keeper.SetTssKeysignFailVoter(ctx, item)
	}

	for _, item := range data.KeygenBlocks {
		if item.IsEmpty() {
			continue
		}
		keeper.SetKeygenBlock(ctx, item)
	}

	for _, item := range data.ErrataTxVoters {
		if item.Empty() {
			continue
		}
		keeper.SetErrataTxVoter(ctx, item)
	}

	for i, item := range data.MsgSwaps {
		if err := keeper.SetSwapQueueItem(ctx, item, i); err != nil {
			panic(err)
		}
	}
	for _, nf := range data.NetworkFees {
		if err := keeper.SaveNetworkFee(ctx, nf.Chain, nf); err != nil {
			panic(err)
		}
	}

	for _, nf := range data.NetworkFeeVoters {
		keeper.SetObservedNetworkFeeVoter(ctx, nf)
	}

	for _, cc := range data.ChainContracts {
		keeper.SetChainContract(ctx, cc)
	}

	// Mint coins into the reserve
	if data.Reserve > 0 {
		coin := common.NewCoin(common.RuneNative, cosmos.NewUint(data.Reserve))
		if err := keeper.MintToModule(ctx, ModuleName, coin); err != nil {
			panic(err)
		}
		if err := keeper.SendFromModuleToModule(ctx, ModuleName, ReserveName, common.NewCoins(coin)); err != nil {
			panic(err)
		}
	}

	for _, admin := range ADMINS {
		addr, err := cosmos.AccAddressFromBech32(admin)
		if err != nil {
			panic(err)
		}
		mimir, _ := common.NewAsset("THOR.MIMIR")
		coin := common.NewCoin(mimir, cosmos.NewUint(1000*common.One))
		// mint some gas asset
		err = keeper.MintToModule(ctx, ModuleName, coin)
		if err != nil {
			panic(err)
		}
		if err := keeper.SendFromModuleToAccount(ctx, ModuleName, addr, common.NewCoins(coin)); err != nil {
			panic(err)
		}
	}

	reserveAddr, _ := keeper.GetModuleAddress(ReserveName)
	ctx.Logger().Info("Reserve Module", "address", reserveAddr.String())
	bondAddr, _ := keeper.GetModuleAddress(BondName)
	ctx.Logger().Info("Bond    Module", "address", bondAddr.String())
	asgardAddr, _ := keeper.GetModuleAddress(AsgardName)
	ctx.Logger().Info("Asgard  Module", "address", asgardAddr.String())

	return validators
}

func getLiquidityProviders(ctx cosmos.Context, k keeper.Keeper, asset common.Asset) LiquidityProviders {
	liquidity_providers := make(LiquidityProviders, 0)
	iterator := k.GetLiquidityProviderIterator(ctx, asset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var lp LiquidityProvider
		k.Cdc().MustUnmarshalBinaryBare(iterator.Value(), &lp)
		liquidity_providers = append(liquidity_providers, lp)
	}
	return liquidity_providers
}

// ExportGenesis export the data in Genesis
func ExportGenesis(ctx cosmos.Context, k keeper.Keeper) GenesisState {
	var iterator cosmos.Iterator

	pools, err := k.GetPools(ctx)
	if err != nil {
		panic(err)
	}

	var liquidity_providers LiquidityProviders
	for _, pool := range pools {
		liquidity_providers = append(liquidity_providers, getLiquidityProviders(ctx, k, pool.Asset)...)
	}

	var nodeAccounts NodeAccounts
	iterator = k.GetNodeAccountIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var na NodeAccount
		k.Cdc().MustUnmarshalBinaryBare(iterator.Value(), &na)
		nodeAccounts = append(nodeAccounts, na)
	}

	var observedTxInVoters ObservedTxVoters
	iterator = k.GetObservedTxInVoterIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var vote ObservedTxVoter
		k.Cdc().MustUnmarshalBinaryBare(iterator.Value(), &vote)
		observedTxInVoters = append(observedTxInVoters, vote)
	}

	var observedTxOutVoters ObservedTxVoters
	iterator = k.GetObservedTxOutVoterIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var vote ObservedTxVoter
		k.Cdc().MustUnmarshalBinaryBare(iterator.Value(), &vote)
		observedTxOutVoters = append(observedTxOutVoters, vote)
	}

	var outs []TxOut
	iterator = k.GetTxOutIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var out TxOut
		k.Cdc().MustUnmarshalBinaryBare(iterator.Value(), &out)
		outs = append(outs, out)
	}

	banVoters := make([]BanVoter, 0)
	iteratorBanVoters := k.GetBanVoterIterator(ctx)
	defer iteratorBanVoters.Close()
	for ; iteratorBanVoters.Valid(); iteratorBanVoters.Next() {
		var bv BanVoter
		k.Cdc().MustUnmarshalBinaryBare(iteratorBanVoters.Value(), &bv)
		banVoters = append(banVoters, bv)
	}

	lastSignedHeight, err := k.GetLastSignedHeight(ctx)
	if err != nil {
		panic(err)
	}

	chainHeights, err := k.GetLastChainHeights(ctx)
	if err != nil {
		panic(err)
	}
	lastChainHeights := make(map[string]int64, 0)
	for k, v := range chainHeights {
		lastChainHeights[k.String()] = v
	}

	reserveContributors, err := k.GetReservesContributors(ctx)
	if err != nil {
		// panic(err)
	}

	network, err := k.GetNetwork(ctx)
	if err != nil {
		panic(err)
	}

	vaults := make(Vaults, 0)
	iterVault := k.GetVaultIterator(ctx)
	defer iterVault.Close()
	for ; iterVault.Valid(); iterVault.Next() {
		var vault Vault
		k.Cdc().MustUnmarshalBinaryBare(iterVault.Value(), &vault)
		vaults = append(vaults, vault)
	}

	tssVoters := make([]TssVoter, 0)
	iterTssVoter := k.GetTssVoterIterator(ctx)
	defer iterTssVoter.Close()
	for ; iterTssVoter.Valid(); iterTssVoter.Next() {
		var tv TssVoter
		k.Cdc().MustUnmarshalBinaryBare(iterTssVoter.Value(), &tv)
		tssVoters = append(tssVoters, tv)
	}

	tssKeySignFailVoters := make([]TssKeysignFailVoter, 0)
	iterTssKeysignFailVoter := k.GetTssKeysignFailVoterIterator(ctx)
	defer iterTssKeysignFailVoter.Close()
	for ; iterTssKeysignFailVoter.Valid(); iterTssKeysignFailVoter.Next() {
		var t TssKeysignFailVoter
		k.Cdc().MustUnmarshalBinaryBare(iterTssKeysignFailVoter.Value(), &t)
		tssKeySignFailVoters = append(tssKeySignFailVoters, t)
	}

	keygenBlocks := make([]KeygenBlock, 0)
	iterKeygenBlocks := k.GetKeygenBlockIterator(ctx)
	for ; iterKeygenBlocks.Valid(); iterKeygenBlocks.Next() {
		var kb KeygenBlock
		k.Cdc().MustUnmarshalBinaryBare(iterKeygenBlocks.Value(), &kb)
		keygenBlocks = append(keygenBlocks, kb)
	}

	errataVoters := make([]ErrataTxVoter, 0)
	iterErrata := k.GetErrataTxVoterIterator(ctx)
	defer iterErrata.Close()
	for ; iterErrata.Valid(); iterErrata.Next() {
		var et ErrataTxVoter
		k.Cdc().MustUnmarshalBinaryBare(iterErrata.Value(), &et)
		errataVoters = append(errataVoters, et)
	}

	swapMsgs := make([]MsgSwap, 0)
	iterMsgSwap := k.GetSwapQueueIterator(ctx)
	defer iterMsgSwap.Close()
	for ; iterMsgSwap.Valid(); iterMsgSwap.Next() {
		var m MsgSwap
		k.Cdc().MustUnmarshalBinaryBare(iterMsgSwap.Value(), &m)
		swapMsgs = append(swapMsgs, m)
	}

	networkFees := make([]NetworkFee, 0)
	iterNetworkFee := k.GetNetworkFeeIterator(ctx)
	defer iterNetworkFee.Close()
	for ; iterNetworkFee.Valid(); iterNetworkFee.Next() {
		var nf NetworkFee
		k.Cdc().MustUnmarshalBinaryBare(iterNetworkFee.Value(), &nf)
		networkFees = append(networkFees, nf)
	}

	networkFeeVoters := make([]ObservedNetworkFeeVoter, 0)
	iterNetworkFeeVoter := k.GetObservedNetworkFeeVoterIterator(ctx)
	defer iterNetworkFeeVoter.Close()
	for ; iterNetworkFeeVoter.Valid(); iterNetworkFeeVoter.Next() {
		var nf ObservedNetworkFeeVoter
		k.Cdc().MustUnmarshalBinaryBare(iterNetworkFeeVoter.Value(), &nf)
		networkFeeVoters = append(networkFeeVoters, nf)
	}
	chainContracts := make([]ChainContract, 0)
	iter := k.GetChainContractIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var cc ChainContract
		k.Cdc().MustUnmarshalBinaryBare(iter.Value(), &cc)
		chainContracts = append(chainContracts, cc)
	}

	return GenesisState{
		Pools:                pools,
		LiquidityProviders:   liquidity_providers,
		ObservedTxInVoters:   observedTxInVoters,
		ObservedTxOutVoters:  observedTxOutVoters,
		TxOuts:               outs,
		NodeAccounts:         nodeAccounts,
		Vaults:               vaults,
		BanVoters:            banVoters,
		LastSignedHeight:     lastSignedHeight,
		LastChainHeights:     lastChainHeights,
		ReserveContributors:  reserveContributors,
		Network:              network,
		TssVoters:            tssVoters,
		TssKeysignFailVoters: tssKeySignFailVoters,
		KeygenBlocks:         keygenBlocks,
		ErrataTxVoters:       errataVoters,
		MsgSwaps:             swapMsgs,
		NetworkFees:          networkFees,
		NetworkFeeVoters:     networkFeeVoters,
	}
}
