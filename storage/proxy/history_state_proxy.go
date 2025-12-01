/*
*   Copyright (c) 2025 Arcology Network

*   This program is free software: you can redistribute it and/or modify
*   it under the terms of the GNU General Public License as published by
*   the Free Software Foundation, either version 3 of the License, or
*   (at your option) any later version.

*   This program is distributed in the hope that it will be useful,
*   but WITHOUT ANY WARRANTY; without even the implied warranty of
*   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
*   GNU General Public License for more details.

*   You should have received a copy of the GNU General Public License
*   along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */
package proxy

import (
	"math"

	ethstg "github.com/arcology-network/state-engine/storage/ethstorage"
	livecache "github.com/arcology-network/state-engine/storage/execstorage/livecache"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

// HistoricalExecStorage provides a replay-based execution-time storage view
// for historical blocks. This storage view is mainly used to support read-only
// historical execution APIs such as eth_call, debug_traceTransaction, and other
// trace-based queries.
//
// It does NOT represent live execution state and must never be used for
// consensus or block production.
type HistoryEthStorageProxy struct {
	*ethstg.EthWorldState
	root        ethcommon.Hash
	blockNumber uint64
	execCache   *livecache.LiveCache // An object cache for the backend storage, only updated once at the end of the block.
}

func NewHistoryEthStorageProxy(root ethcommon.Hash, blockNumber uint64, backend *ethstg.EthShardTrieDB) (*HistoryEthStorageProxy, error) {
	ethWorldState, err := ethstg.LoadEthTrieByRoot(backend.MainTrieDB(), root)
	if err != nil {
		return nil, err
	}

	return &HistoryEthStorageProxy{
		EthWorldState: ethWorldState,
		root:          root,
		blockNumber:   blockNumber,
		execCache:     livecache.NewLiveCache(math.MaxUint64), // All the data in the execution cache will be removed after the call.
	}, nil
}

// Check if the key exists in the source, which can be a cache or a storage.
func (this *HistoryEthStorageProxy) IfExists(key string) bool {
	if _, ok := this.execCache.Get(key); ok { // Check the cache first
		return true
	}
	return this.EthWorldState.IfExists(key)
}

func (this *HistoryEthStorageProxy) ReadStorage(key string, T any) (any, error) {
	if v, ok := this.execCache.Get(key); ok { // Check the cache first
		return v, nil
	}
	return this.EthWorldState.Retrieve(key, T)
}

func (this *HistoryEthStorageProxy) Retrieve(key string, v any) (any, error) {
	return this.ReadStorage(key, v)
}

func (this *HistoryEthStorageProxy) Preload(data []byte) any {
	return this.EthWorldState.Preload(data)
}

func (this *HistoryEthStorageProxy) Close() error { return this.EthWorldState.Close() }
