package thorchain

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	sdkRest "github.com/cosmos/cosmos-sdk/x/auth/client/rest"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
	abci "github.com/tendermint/tendermint/abci/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	"gitlab.com/thorchain/thornode/x/thorchain/client/cli"
	"gitlab.com/thorchain/thornode/x/thorchain/client/rest"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// type check to ensure the interface is properly implemented
var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
)

// AppModuleBasic app module Basics object
type AppModuleBasic struct{}

// Name return the module's name
func (AppModuleBasic) Name() string {
	return ModuleName
}

// RegisterLegacyAminoCodec registers the module's types for the given codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	RegisterCodec(cdc)
}

// RegisterInterfaces registers the module's interface types
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	RegisterInterfaces(reg)
}

// DefaultGenesis returns default genesis state as raw bytes for the module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONMarshaler) json.RawMessage {
	return cdc.MustMarshalJSON(DefaultGenesis())
}

// ValidateGenesis check of the Genesis
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONMarshaler, config client.TxEncodingConfig, bz json.RawMessage) error {
	var data GenesisState
	if err := cdc.UnmarshalJSON(bz, &data); err != nil {
		return err
	}
	// Once json successfully marshalled, passes along to genesis.go
	return ValidateGenesis(data)
}

// RegisterRESTRoutes register rest routes
func (AppModuleBasic) RegisterRESTRoutes(ctx client.Context, rtr *mux.Router) {
	rest.RegisterRoutes(ctx, rtr, StoreKey)
	sdkRest.RegisterTxRoutes(ctx, rtr)
	sdkRest.RegisterRoutes(ctx, rtr, StoreKey)
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the mint module.
// thornode current doesn't have grpc enpoint yet
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	// types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))
}

// GetQueryCmd get the root query command of this module
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// GetTxCmd get the root tx command of this module
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// ____________________________________________________________________________

// AppModule implements an application module for the thorchain module.
type AppModule struct {
	AppModuleBasic
	keeper             keeper.Keeper
	coinKeeper         bankkeeper.Keeper
	mgr                *Mgrs
	keybaseStore       cosmos.KeybaseStore
	existingValidators []string
}

// NewAppModule creates a new AppModule Object
func NewAppModule(k keeper.Keeper, bankKeeper bankkeeper.Keeper) AppModule {
	kb, err := cosmos.GetKeybase(os.Getenv("CHAIN_HOME_FOLDER"))
	if err != nil {
		panic(err)
	}
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         k,
		coinKeeper:     bankKeeper,
		mgr:            NewManagers(k),
		keybaseStore:   kb,
	}
}

func (AppModule) Name() string {
	return ModuleName
}

func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {}

func (am AppModule) Route() cosmos.Route {
	return cosmos.NewRoute(RouterKey, NewExternalHandler(am.keeper, am.mgr))
}

func (am AppModule) NewHandler() sdk.Handler {
	return NewExternalHandler(am.keeper, am.mgr)
}

func (am AppModule) QuerierRoute() string {
	return ModuleName
}

// LegacyQuerierHandler returns the capability module's Querier.
func (am AppModule) LegacyQuerierHandler(legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {
	return NewQuerier(am.keeper, am.keybaseStore)
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	// types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	// types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

func (am AppModule) NewQuerierHandler() sdk.Querier {
	return func(ctx cosmos.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		return nil, nil
	}
}

// BeginBlock called when a block get proposed
func (am AppModule) BeginBlock(ctx sdk.Context, req abci.RequestBeginBlock) {
	info := req.GetLastCommitInfo()
	var existingValidators []string
	for _, v := range info.GetVotes() {
		addr := sdk.ValAddress(v.Validator.GetAddress())
		existingValidators = append(existingValidators, addr.String())
	}
	am.existingValidators = existingValidators
	ctx.Logger().Debug("Begin Block", "height", req.Header.Height)
	version := am.keeper.GetLowestActiveVersion(ctx)

	// Does a kvstore migration
	smgr := NewStoreMgr(am.keeper)
	if err := smgr.Iterator(ctx); err != nil {
		os.Exit(10) // halt the chain if unsuccessful
	}

	am.keeper.ClearObservingAddresses(ctx)
	if err := am.mgr.BeginBlock(ctx); err != nil {
		ctx.Logger().Error("fail to get managers", "error", err)
	}
	am.mgr.GasMgr().BeginBlock()

	constantValues := constants.GetConstantValues(version)
	if constantValues == nil {
		ctx.Logger().Error(fmt.Sprintf("constants for version(%s) is not available", version))
		return
	}

	am.mgr.Slasher().BeginBlock(ctx, req, constantValues)

	if err := am.mgr.ValidatorMgr().BeginBlock(ctx, constantValues); err != nil {
		ctx.Logger().Error("Fail to begin block on validator", "error", err)
	}
}

// EndBlock called when a block get committed
func (am AppModule) EndBlock(ctx sdk.Context, req abci.RequestEndBlock) []abci.ValidatorUpdate {
	ctx.Logger().Debug("End Block", "height", req.Height)
	version := am.keeper.GetLowestActiveVersion(ctx)
	constantValues := constants.GetConstantValues(version)
	if constantValues == nil {
		ctx.Logger().Error(fmt.Sprintf("constants for version(%s) is not available", version))
		return nil
	}
	if err := am.mgr.SwapQ().EndBlock(ctx, am.mgr, version, constantValues); err != nil {
		ctx.Logger().Error("fail to process swap queue", "error", err)
	}

	// slash node accounts for not observing any accepted inbound tx
	if err := am.mgr.Slasher().LackObserving(ctx, constantValues); err != nil {
		ctx.Logger().Error("Unable to slash for lack of observing:", "error", err)
	}
	if err := am.mgr.Slasher().LackSigning(ctx, constantValues, am.mgr); err != nil {
		ctx.Logger().Error("Unable to slash for lack of signing:", "error", err)
	}

	poolCycle, err := am.keeper.GetMimir(ctx, constants.PoolCycle.String())
	if poolCycle < 0 || err != nil {
		poolCycle = constantValues.GetInt64Value(constants.PoolCycle)
	}
	// Enable a pool every poolCycle
	if common.BlockHeight(ctx)%poolCycle == 0 && !am.keeper.RagnarokInProgress(ctx) {
		maxAvailablePools, err := am.keeper.GetMimir(ctx, constants.MaxAvailablePools.String())
		if maxAvailablePools < 0 || err != nil {
			maxAvailablePools = constantValues.GetInt64Value(constants.MaxAvailablePools)
		}
		minRunePoolDepth, err := am.keeper.GetMimir(ctx, constants.MinRunePoolDepth.String())
		if minRunePoolDepth < 0 || err != nil {
			minRunePoolDepth = constantValues.GetInt64Value(constants.MinRunePoolDepth)
		}
		stagedPoolCost, err := am.keeper.GetMimir(ctx, constants.StagedPoolCost.String())
		if stagedPoolCost < 0 || err != nil {
			stagedPoolCost = constantValues.GetInt64Value(constants.StagedPoolCost)
		}
		if err := cyclePools(ctx, maxAvailablePools, minRunePoolDepth, stagedPoolCost, am.keeper, am.mgr.EventMgr()); err != nil {
			ctx.Logger().Error("Unable to enable a pool", "error", err)
		}
	}

	am.mgr.ObMgr().EndBlock(ctx, am.keeper)

	// update network data to account for block rewards and reward units
	if err := am.mgr.VaultMgr().UpdateNetwork(ctx, constantValues, am.mgr.GasMgr(), am.mgr.EventMgr()); err != nil {
		ctx.Logger().Error("fail to update network data", "error", err)
	}

	if err := am.mgr.VaultMgr().EndBlock(ctx, am.mgr, constantValues); err != nil {
		ctx.Logger().Error("fail to end block for vault manager", "error", err)
	}

	validators := am.mgr.ValidatorMgr().EndBlock(ctx, am.mgr, constantValues, am.existingValidators)

	// Fill up Yggdrasil vaults
	// We do this AFTER validatorMgr.EndBlock, because we don't want to send
	// funds to a yggdrasil vault that is being churned out this block.
	if err := am.mgr.YggManager().Fund(ctx, am.mgr, constantValues); err != nil {
		ctx.Logger().Error("unable to fund yggdrasil", "error", err)
	}

	am.mgr.GasMgr().EndBlock(ctx, am.keeper, am.mgr.EventMgr())

	return validators
}

// InitGenesis initialise genesis
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONMarshaler, data json.RawMessage) []abci.ValidatorUpdate {
	var genState GenesisState
	ModuleCdc.MustUnmarshalJSON(data, &genState)
	return InitGenesis(ctx, am.keeper, genState)
}

// ExportGenesis export genesis
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONMarshaler) json.RawMessage {
	gs := ExportGenesis(ctx, am.keeper)
	return ModuleCdc.MustMarshalJSON(&gs)
}
