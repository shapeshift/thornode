package types

import (
	"errors"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type ReserveContributor struct {
	Address common.Address `json:"address"`
	Amount  cosmos.Uint    `json:"amount"`
}

type ReserveContributors []ReserveContributor

func NewReserveContributor(addr common.Address, amt cosmos.Uint) ReserveContributor {
	return ReserveContributor{
		Address: addr,
		Amount:  amt,
	}
}

func (res ReserveContributor) IsEmpty() bool {
	return res.Address.IsEmpty()
}

// IsValid check whether reserve contributor has all necessary values
func (res ReserveContributor) IsValid() error {
	if res.Amount.IsZero() {
		return errors.New("amount cannot be zero")
	}
	if res.Address.IsEmpty() {
		return errors.New("address cannot be empty")
	}
	return nil
}

func (reses ReserveContributors) Add(res ReserveContributor) ReserveContributors {
	for i, r := range reses {
		if r.Address.Equals(res.Address) {
			reses[i].Amount = reses[i].Amount.Add(res.Amount)
			return reses
		}
	}

	return append(reses, res)
}
