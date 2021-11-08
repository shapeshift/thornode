package litecoin

import (
	"encoding/json"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// PrefixUTXOStorage declares prefix to use in leveldb to avoid conflicts
const (
	TransactionFeeKey = "transactionfee-"
	PrefixBlockMeta   = `blockmeta-`
	PrefixMempool     = "mempool-"
	PrefixObservedTx  = "observed-"
)

// LevelDBBlockMetaAccessor struct
type LevelDBBlockMetaAccessor struct {
	db *leveldb.DB
}

// NewLevelDBBlockMetaAccessor creates a new level db backed BlockMeta accessor
func NewLevelDBBlockMetaAccessor(db *leveldb.DB) (*LevelDBBlockMetaAccessor, error) {
	return &LevelDBBlockMetaAccessor{db: db}, nil
}

func (t *LevelDBBlockMetaAccessor) getBlockMetaKey(height int64) string {
	return fmt.Sprintf(PrefixBlockMeta+"%d", height)
}
func (t *LevelDBBlockMetaAccessor) getMemPoolHashKey(hash string) string {
	return PrefixMempool + hash
}
func (t *LevelDBBlockMetaAccessor) getObservedTxHashKey(hash string) string {
	return PrefixObservedTx + hash
}

// GetBlockMeta at given block height ,  when the requested block meta doesn't exist , it will return nil , thus caller need to double check it
func (t *LevelDBBlockMetaAccessor) GetBlockMeta(height int64) (*BlockMeta, error) {
	key := t.getBlockMetaKey(height)
	exist, err := t.db.Has([]byte(key), nil)
	if err != nil {
		return nil, fmt.Errorf("fail to check whether block meta(%s) exist: %w", key, err)
	}
	if !exist {
		return nil, nil
	}
	v, err := t.db.Get([]byte(key), nil)
	if err != nil {
		return nil, fmt.Errorf("fail to get block meta(%s) from storage: %w", key, err)
	}
	var blockMeta BlockMeta
	if err := json.Unmarshal(v, &blockMeta); err != nil {
		return nil, fmt.Errorf("fail to unmarshal block meta from json: %w", err)
	}
	return &blockMeta, nil
}

// SaveBlockMeta persistent the given BlockMeta into storage
func (t *LevelDBBlockMetaAccessor) SaveBlockMeta(height int64, blockMeta *BlockMeta) error {
	key := t.getBlockMetaKey(height)
	buf, err := json.Marshal(blockMeta)
	if err != nil {
		return fmt.Errorf("fail to marshal block meta to json: %w", err)
	}
	return t.db.Put([]byte(key), buf, nil)
}

// GetBlockMetas returns all the block metas in storage
// The chain client will Prune block metas every time it finished scan a block , so at maximum it will keep BlockCacheSize blocks
// thus it should not grow out of control
func (t *LevelDBBlockMetaAccessor) GetBlockMetas() ([]*BlockMeta, error) {
	blockMetas := make([]*BlockMeta, 0)
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(PrefixBlockMeta)), nil)
	defer iterator.Release()
	for ; iterator.Next(); iterator.Valid() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var blockMeta BlockMeta
		if err := json.Unmarshal(buf, &blockMeta); err != nil {
			return nil, fmt.Errorf("fail to unmarshal block meta: %w", err)
		}
		blockMetas = append(blockMetas, &blockMeta)
	}
	return blockMetas, nil
}

// PruneBlockMeta remove all block meta that is older than the given block height
// with exception, if there are unspent transaction output in it , then the block meta will not be removed
func (t *LevelDBBlockMetaAccessor) PruneBlockMeta(height int64, callback PruneBlockMetaCallback) error {
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(PrefixBlockMeta)), nil)
	defer iterator.Release()
	targetToDelete := make([]string, 0)
	for ; iterator.Next(); iterator.Valid() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var blockMeta BlockMeta
		if err := json.Unmarshal(buf, &blockMeta); err != nil {
			return fmt.Errorf("fail to unmarshal block meta: %w", err)
		}

		if blockMeta.Height < height {
			if callback != nil && !callback(&blockMeta) {
				continue
			}
			targetToDelete = append(targetToDelete, t.getBlockMetaKey(blockMeta.Height))
		}
	}

	for _, key := range targetToDelete {
		if err := t.db.Delete([]byte(key), nil); err != nil {
			return fmt.Errorf("fail to delete block meta with key(%s) from storage: %w", key, err)
		}
	}
	return nil
}

// UpsertTransactionFee update the transaction fee in storage
func (t *LevelDBBlockMetaAccessor) UpsertTransactionFee(fee float64, vSize int32) error {
	transactionFee := TransactionFee{
		Fee:   fee,
		VSize: vSize,
	}
	buf, err := json.Marshal(transactionFee)
	if err != nil {
		return fmt.Errorf("fail to marshal transaction fee struct to json: %w", err)
	}
	return t.db.Put([]byte(TransactionFeeKey), buf, nil)
}

// GetTransactionFee from db
func (t *LevelDBBlockMetaAccessor) GetTransactionFee() (float64, int32, error) {
	buf, err := t.db.Get([]byte(TransactionFeeKey), nil)
	if err != nil {
		return 0.0, 0, fmt.Errorf("fail to get transaction fee from storage: %w", err)
	}
	var transactionFee TransactionFee
	if err := json.Unmarshal(buf, &transactionFee); err != nil {
		return 0.0, 0, fmt.Errorf("fail to unmarshal transaction fee: %w", err)
	}
	return transactionFee.Fee, transactionFee.VSize, nil
}

// TryAddToMemPoolCache trying to add the given hash to mempool hash
// return value bool indicated whether we added value to cache or not
func (t *LevelDBBlockMetaAccessor) TryAddToMemPoolCache(hash string) (bool, error) {
	key := t.getMemPoolHashKey(hash)
	exist, err := t.db.Has([]byte(key), nil)
	if err != nil {
		return exist, err
	}
	if exist {
		return false, nil
	}
	err = t.db.Put([]byte(key), []byte(hash), nil)
	return true, err
}

// RemoveFromMemPoolCache remove the hash from cache
func (t *LevelDBBlockMetaAccessor) RemoveFromMemPoolCache(hash string) error {
	key := t.getMemPoolHashKey(hash)
	return t.db.Delete([]byte(key), nil)
}

// TryAddToObservedTxCache store the transaction hash into local key value store
// so as bifrost will not report a tx twice, which will cause it to be slashed by thornode
func (t *LevelDBBlockMetaAccessor) TryAddToObservedTxCache(hash string) (bool, error) {
	key := t.getObservedTxHashKey(hash)
	exist, err := t.db.Has([]byte(key), nil)
	if err != nil {
		return exist, err
	}
	if exist {
		return false, nil
	}
	err = t.db.Put([]byte(key), []byte(hash), nil)
	return true, err
}

// RemoveObservedTxCache remove the hash from cache
func (t *LevelDBBlockMetaAccessor) RemoveObservedTxCache(hash string) error {
	key := t.getObservedTxHashKey(hash)
	return t.db.Delete([]byte(key), nil)
}
