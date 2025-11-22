/*
*   Copyright (c) 2023 Arcology Network
*   All rights reserved.

*   Licensed under the Apache License, Version 2.0 (the "License");
*   you may not use this file except in compliance with the License.
*   You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*   Unless required by applicable law or agreed to in writing, software
*   distributed under the License is distributed on an "AS IS" BASIS,
*   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*   See the License for the specific language governing permissions and
*   limitations under the License.
 */
package cache

import (
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
	mapi "github.com/arcology-network/common-lib/exp/map"
	slice "github.com/arcology-network/common-lib/exp/slice"

	// stgcommon "github.com/arcology-network/state-engine/common"
	stgcommon "github.com/arcology-network/state-engine/common"
)

// StateCacheFilter is a post processing filter for StateCache.
// It is used to filter out the transitions based on the addresses.
// out the transitions based on the addresses.
type StateCacheFilter struct {
	*StateCache
	ignoreAddresses map[string]bool
}

func NewStateCacheFilter(writeCache any) *StateCacheFilter {
	return &StateCacheFilter{
		writeCache.(*StateCache),
		map[string]bool{},
	}
}

func (this *StateCacheFilter) ToBuffer() []*statecell.StateCell {
	return mapi.Values(*this.StateCache.Cache())
}

func (this *StateCacheFilter) RemoveByAddress(addr string) {
	mapi.RemoveIf(this.kvDict,
		func(path string, _ *statecell.StateCell) bool {
			return path[stgcommon.ETH_ACCOUNT_PREFIX_LENGTH:stgcommon.ETH_ACCOUNT_PREFIX_LENGTH+stgcommon.ETH_ACCOUNT_LENGTH] == addr
		},
	)
}

func (this *StateCacheFilter) AddToAutoReversion(addr string) {
	if _, ok := (this.ignoreAddresses)[addr]; !ok {
		(this.ignoreAddresses)[addr] = true
	}
}

func (this *StateCacheFilter) filterByAddress(transitions *[]*statecell.StateCell) []*statecell.StateCell {
	if len(this.ignoreAddresses) == 0 {
		return *transitions
	}

	out := slice.RemoveIf(transitions, func(_ int, v *statecell.StateCell) bool {
		address := (*v.GetPath())[stgcommon.ETH_ACCOUNT_PREFIX_LENGTH : stgcommon.ETH_ACCOUNT_PREFIX_LENGTH+stgcommon.ETH_ACCOUNT_LENGTH]
		_, ok := this.ignoreAddresses[address]
		return ok
	})

	return out
}

func (this *StateCacheFilter) ByType() ([]*statecell.StateCell, []*statecell.StateCell) {
	accesses, transitions := this.ExportAll()
	return this.filterByAddress(&accesses), this.filterByAddress(&transitions)
}
