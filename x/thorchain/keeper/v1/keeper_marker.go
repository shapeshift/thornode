package keeperv1

import (
	"errors"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (k KVStore) setTxMarkers(ctx cosmos.Context, key string, record TxMarkers) {
	return
	/*
		store := ctx.KVStore(k.storeKey)
		buf := k.cdc.MustMarshalBinaryBare(&record)
		if buf == nil {
			store.Delete([]byte(key))
		} else {
			store.Set([]byte(key), buf)
		}
	*/
}

func (k KVStore) getTxMarkers(ctx cosmos.Context, key string, record *TxMarkers) (bool, error) {
	return true, nil
	/*
		store := ctx.KVStore(k.storeKey)
		if !store.Has([]byte(key)) {
			return false, nil
		}

		bz := store.Get([]byte(key))
		if err := k.cdc.UnmarshalBinaryBare(bz, record); err != nil {
			return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
		}
		return true, nil
	*/
}

// ListTxMarker get all tx marker related to the given hash
func (k KVStore) ListTxMarker(ctx cosmos.Context, hash string) (TxMarkers, error) {
	record := make(TxMarkers, 0)
	_, err := k.getTxMarkers(ctx, k.GetKey(ctx, prefixSupportedTxMarker, hash), &record)
	return record, err
}

// SetTxMarkers save the given tx markers again the given hash
func (k KVStore) SetTxMarkers(ctx cosmos.Context, hash string, orig TxMarkers) error {
	marks := make(TxMarkers, 0)
	for _, mark := range orig {
		if !mark.IsEmpty() {
			marks = append(marks, mark)
		}
	}

	k.setTxMarkers(ctx, k.GetKey(ctx, prefixSupportedTxMarker, hash), marks)
	return nil
}

// AppendTxMarker append the given tx marker to store
func (k KVStore) AppendTxMarker(ctx cosmos.Context, hash string, mark TxMarker) error {
	if mark.IsEmpty() {
		return dbError(ctx, "unable to save tx marker:", errors.New("is empty"))
	}
	marks, err := k.ListTxMarker(ctx, hash)
	if err != nil {
		return err
	}
	marks = append(marks, mark)
	k.setTxMarkers(ctx, k.GetKey(ctx, prefixSupportedTxMarker, hash), marks)
	return nil
}

// GetAllTxMarkers get all tx markers from key value store
func (k KVStore) GetAllTxMarkers(ctx cosmos.Context) (map[string]TxMarkers, error) {
	result := make(map[string]TxMarkers)
	iter := k.getIterator(ctx, prefixSupportedTxMarker)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		/*
			var marker TxMarkers
			if err := k.cdc.UnmarshalBinaryBare(iter.Value(), &marker); err != nil {
				return nil, fmt.Errorf("fail to unmarshal tx marker: %w", err)
			}

			strKey := string(iter.Key())
			k := strings.TrimPrefix(strKey, string(prefixSupportedTxMarker+"/"))
			result[k] = marker
		*/
	}
	return result, nil
}
