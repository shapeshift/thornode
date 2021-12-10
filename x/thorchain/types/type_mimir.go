package types

import (
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (m NodeMimirs) Has(acc cosmos.AccAddress) bool {
	for _, mim := range m.Mimirs {
		if mim.Signer.Equals(acc) {
			return true
		}
	}
	return false
}

func (m NodeMimirs) Get(acc cosmos.AccAddress) (int64, bool) {
	for _, mim := range m.Mimirs {
		if mim.Signer.Equals(acc) {
			return mim.Value, true
		}
	}
	return 0, false
}

func (m *NodeMimirs) Set(acc cosmos.AccAddress, key string, val int64) {
	for i, mim := range m.Mimirs {
		if mim.Key == key && mim.Signer.Equals(acc) {
			m.Mimirs[i].Value = val
			return
		}
	}
	m.Mimirs = append(m.Mimirs, NodeMimir{
		Key:    key,
		Value:  val,
		Signer: acc,
	})
}

func (m *NodeMimirs) Delete(acc cosmos.AccAddress, key string) {
	for i, mim := range m.Mimirs {
		if mim.Signer.Equals(acc) {
			m.Mimirs = append(m.Mimirs[:i], m.Mimirs[i+1:]...)
			return
		}
	}
}

func (m NodeMimirs) countActive(key string, active []cosmos.AccAddress, maj func(_, _ int) bool) (int64, bool) {
	counter := make(map[int64]int, 0) // count how many votes are for each value
	voted := make(map[string]bool, 0) // track signers that have already voted
	for _, mimir := range m.Mimirs {
		// skip mismatching keys
		if key != mimir.Key {
			continue
		}

		// skip signers we've already seend (no duplicates allowed)
		if v, ok := voted[mimir.Signer.String()]; v && ok {
			continue
		}

		for _, acc := range active {
			// skip if not an active signer
			if !acc.Equals(mimir.Signer) {
				continue
			}

			voted[mimir.Signer.String()] = true // mark signer as voted
			if _, ok := counter[mimir.Value]; !ok {
				counter[mimir.Value] = 0
			}
			counter[mimir.Value]++
		}
	}

	for val, count := range counter {
		if maj(count, len(active)) {
			return val, true
		}
	}

	return 0, false
}

func (m NodeMimirs) HasSuperMajority(key string, nas []cosmos.AccAddress) (int64, bool) {
	return m.countActive(key, nas, HasSuperMajority)
}

func (m NodeMimirs) HasSimpleMajority(key string, nas []cosmos.AccAddress) (int64, bool) {
	return m.countActive(key, nas, HasSimpleMajority)
}

func (m NodeMimirs) HasMinority(key string, nas []cosmos.AccAddress) (int64, bool) {
	// NOT IMPLEMENTED
	// Minotirty is a bit tricky, because a set can have multiple minorities, which can result in a potential consensus failure
	return 0, false
}
