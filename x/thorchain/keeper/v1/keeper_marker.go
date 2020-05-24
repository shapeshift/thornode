package thorchain

import (
	"errors"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStoreV1) ListTxMarker(ctx cosmos.Context, hash string) (TxMarkers, error) {
	marks := make(TxMarkers, 0)
	key := k.GetKey(ctx, prefixSupportedTxMarker, hash)
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return marks, nil
	}
	buf := store.Get([]byte(key))
	err := k.cdc.UnmarshalBinaryBare(buf, &marks)
	if err != nil {
		return marks, dbError(ctx, "Unmarshal: tx markers", err)
	}
	return marks, nil
}

func (k KVStoreV1) SetTxMarkers(ctx cosmos.Context, hash string, orig TxMarkers) error {
	marks := make(TxMarkers, 0)
	for _, mark := range orig {
		if !mark.IsEmpty() {
			marks = append(marks, mark)
		}
	}

	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixSupportedTxMarker, hash)
	if len(marks) == 0 {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(marks))
	}
	return nil
}

func (k KVStoreV1) AppendTxMarker(ctx cosmos.Context, hash string, mark TxMarker) error {
	if mark.IsEmpty() {
		return dbError(ctx, "unable to save tx marker:", errors.New("is empty"))
	}
	marks, err := k.ListTxMarker(ctx, hash)
	if err != nil {
		return err
	}

	marks = append(marks, mark)

	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixSupportedTxMarker, hash)
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(marks))
	return nil
}
