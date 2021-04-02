package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// Valid is to check whether the pool status is valid or not
func (x PoolStatus) Valid() error {
	if _, ok := PoolStatus_value[x.String()]; !ok {
		return errors.New("invalid pool status")
	}
	return nil
}

// MarshalJSON marshal PoolStatus to JSON in string form
func (x PoolStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.String())
}

// UnmarshalJSON convert string form back to PoolStatus
func (x *PoolStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*x = GetPoolStatus(s)
	return nil
}

// GetPoolStatus from string
func GetPoolStatus(ps string) PoolStatus {
	if val, ok := PoolStatus_value[ps]; ok {
		return PoolStatus(val)
	}
	return PoolStatus_Suspended
}

// Pools represent a list of pools
type Pools []Pool

// NewPool Returns a new Pool
func NewPool() Pool {
	return Pool{
		BalanceRune:         cosmos.ZeroUint(),
		BalanceAsset:        cosmos.ZeroUint(),
		PoolUnits:           cosmos.ZeroUint(),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundRune:  cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		Status:              PoolStatus_Available,
	}
}

// Valid check whether the pool is valid or not, if asset is empty then it is not valid
func (m Pool) Valid() error {
	if m.IsEmpty() {
		return errors.New("pool asset cannot be empty")
	}
	return nil
}

// IsAvailable check whether the pool is in Available status
func (m Pool) IsAvailable() bool {
	return m.Status == PoolStatus_Available
}

// IsEmpty will return true when the asset is empty
func (m Pool) IsEmpty() bool {
	return m.Asset.IsEmpty()
}

// String implement fmt.Stringer
func (m Pool) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintln("rune-balance: " + m.BalanceRune.String()))
	sb.WriteString(fmt.Sprintln("asset-balance: " + m.BalanceAsset.String()))
	sb.WriteString(fmt.Sprintln("asset: " + m.Asset.String()))
	sb.WriteString(fmt.Sprintln("pool-units: " + m.PoolUnits.String()))
	sb.WriteString(fmt.Sprintln("synth-units: " + m.SynthUnits.String()))
	sb.WriteString(fmt.Sprintln("pending-inbound-rune: " + m.PendingInboundRune.String()))
	sb.WriteString(fmt.Sprintln("pending-inbound-asset: " + m.PendingInboundAsset.String()))
	sb.WriteString(fmt.Sprintln("status: " + m.Status.String()))
	sb.WriteString(fmt.Sprintln("decimals:" + strconv.FormatInt(m.Decimals, 10)))
	return sb.String()
}

// EnsureValidPoolStatus make sure the pool is in a valid status otherwise it return an error
func (m Pool) EnsureValidPoolStatus(msg cosmos.Msg) error {
	switch m.Status {
	case PoolStatus_Available:
		return nil
	case PoolStatus_Staged:
		switch msg.Type() {
		case "swap":
			return errors.New("pool is in staged status, can't swap")
		default:
			return nil
		}
	case PoolStatus_Suspended:
		return errors.New("pool suspended")
	default:
		return fmt.Errorf("unknown pool status,%s", m.Status)
	}
}

// AssetValueInRune convert a specific amount of asset amt into its rune value
func (m Pool) AssetValueInRune(amt cosmos.Uint) cosmos.Uint {
	if m.BalanceRune.IsZero() || m.BalanceAsset.IsZero() {
		return cosmos.ZeroUint()
	}
	return common.GetShare(m.BalanceRune, m.BalanceAsset, amt)
}

// RuneValueInAsset convert a specific amount of rune amt into its asset value
func (m Pool) RuneValueInAsset(amt cosmos.Uint) cosmos.Uint {
	if m.BalanceRune.IsZero() || m.BalanceAsset.IsZero() {
		return cosmos.ZeroUint()
	}
	assetAmt := common.GetShare(m.BalanceAsset, m.BalanceRune, amt)
	return cosmos.RoundToDecimal(assetAmt, m.Decimals)
}
