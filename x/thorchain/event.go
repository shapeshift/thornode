package thorchain

// This file is intended to do orchestration for emitting an event

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/thorchain/thornode/common"
)

func eventPoolStatusWrapper(ctx sdk.Context, keeper Keeper, pool Pool) error {
	poolEvt := NewEventPool(pool.Asset, pool.Status)
	bytes, err := json.Marshal(poolEvt)
	if err != nil {
		return fmt.Errorf("fail to marshal pool event: %w", err)
	}
	eventID, err := keeper.GetNextEventID(ctx)
	if nil != err {
		return fmt.Errorf("fail to get next event id: %w", err)
	}
	tx := common.Tx{ID: common.BlankTxID}
	evt := NewEvent(eventID, poolEvt.Type(), ctx.BlockHeight(), tx, bytes, EventSuccess)
	keeper.UpsertEvent(ctx, evt)
	return nil
}
