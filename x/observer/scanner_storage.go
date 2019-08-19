package observer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// ScannerStorage define the method need to be used by scanner
type ScannerStorage interface {
	GetScanPos() (int64, error)
	SetScanPos(block int64) error

	SetBlockScanStatus(block int64, status BlockScanStatus) error
	RemoveBlockStatus(block int64) error

	GetBlocksForRetry(failedOnly bool) ([]int64, error)
	io.Closer
}

type BlockStatusItem struct {
	Height int64           `json:"height"`
	Status BlockScanStatus `json:"status"`
}

// LevelDBScannerStorage is a scanner storage backed by level db
type LevelDBScannerStorage struct {
	db *leveldb.DB
}

const (
	DefaultLevelDBFolder = `data`
	ScanPosKey           = "blockscanpos"
)

// NewLevelDBScannerStorage create a new instance of LevelDBScannerStorage
func NewLevelDBScannerStorage(levelDbFolder string) (*LevelDBScannerStorage, error) {
	if len(levelDbFolder) == 0 {
		levelDbFolder = DefaultLevelDBFolder
	}
	db, err := leveldb.OpenFile(levelDbFolder, nil)
	if nil != err {
		return nil, errors.Wrapf(err, "fail to open level db %s", levelDbFolder)
	}
	return &LevelDBScannerStorage{db: db}, nil
}

// GetScanPos get current Scan Pos
func (ldbss *LevelDBScannerStorage) GetScanPos() (int64, error) {
	buf, err := ldbss.db.Get([]byte(ScanPosKey), nil)
	if nil != err {
		return 0, err
	}
	pos, _ := binary.Varint(buf)
	return pos, nil
}

// SetScanPos save current scan pos
func (ldbss *LevelDBScannerStorage) SetScanPos(block int64) error {
	buf := make([]byte, 8)
	n := binary.PutVarint(buf, block)
	return ldbss.db.Put([]byte(ScanPosKey), buf[:n], nil)
}
func (ldbss *LevelDBScannerStorage) SetBlockScanStatus(block int64, status BlockScanStatus) error {
	blockStatusItem := BlockStatusItem{
		Height: block,
		Status: status,
	}
	buf, err := json.Marshal(blockStatusItem)
	if nil != err {
		return errors.Wrap(err, "fail to marshal BlockStatusItem to json")
	}
	if err := ldbss.db.Put([]byte(getBlockStatusKey(block)), buf, nil); nil != err {
		return errors.Wrap(err, "fail to set block scan status")
	}
	return nil
}

// GetFailedBlocksForRetry
func (ldbss *LevelDBScannerStorage) GetBlocksForRetry(failedOnly bool) ([]int64, error) {
	iterator := ldbss.db.NewIterator(util.BytesPrefix([]byte("block-process-status-")), nil)
	defer iterator.Release()
	var results []int64
	for iterator.Next() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var blockStatusItem BlockStatusItem
		if err := json.Unmarshal(buf, &blockStatusItem); nil != err {
			return nil, errors.Wrap(err, "fail to unmarshal to block status item")
		}
		if !failedOnly {
			results = append(results, blockStatusItem.Height)
			continue
		}
		if blockStatusItem.Status == Failed {
			results = append(results, blockStatusItem.Height)
		}
	}
	return results, nil
}

func getBlockStatusKey(block int64) string {
	return fmt.Sprintf("block-process-status-%d", block)
}
func (ldbss *LevelDBScannerStorage) RemoveBlockStatus(block int64) error {
	return ldbss.db.Delete([]byte(getBlockStatusKey(block)), nil)
}

// Close underlying db
func (ldbss *LevelDBScannerStorage) Close() error {
	return ldbss.db.Close()
}
