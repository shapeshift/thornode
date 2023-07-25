package thorchain

import (
	mem "gitlab.com/thorchain/thornode/x/thorchain/memo"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

var (
	NewEventSwitch    = types.NewEventSwitch
	NewEventSwitchV87 = types.NewEventSwitchV87
	NewMsgSwitch      = types.NewMsgSwitch
)

type (
	MsgSwitch  = types.MsgSwitch
	SwitchMemo = mem.SwitchMemo
)
