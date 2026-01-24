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

package cache

import (
	"bytes"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	statecell "github.com/arcology-network/common-lib/crdt/statecell"
)

// PreloadMatched preloads the paths that match the wildcard delete path that are about to be deleted by the
// the current write operation.
func (this *StateCache) ResolveWildcardDeletion(path string, T crdtcommon.CRDT) (bool, *statecell.StateCell) {
	// Delete only for now.
	for _, wildcardPath := range this.pendingWildcardDeletes {
		if len(path) < len(wildcardPath.Second) {
			continue
		}

		if bytes.Equal([]byte(path[:len(wildcardPath.Second)]), []byte(wildcardPath.Second)) {
			cell := this.LoadFromCommitted(0, path, T) // Preload the path from the backend
			cell.SetValue(nil)                         // To indicate t the path has been deleted by the wildcard
			cell.IncrementWrites(1)
			return true, cell
		}
	}
	return false, nil
}

// WildcardsToUnivalue converts wildcard paths to StateCell for exporting.
func (this *StateCache) WildcardsToStateCell() []*statecell.StateCell {
	univs := make([]*statecell.StateCell, 0)
	for _, wildcardPath := range this.pendingWildcardDeletes {
		newV := statecell.NewStateCell(wildcardPath.First, wildcardPath.Second+"*", 0, 1, 0, nil, nil)
		newV.SetPreexist(true) // Mark as pre-existing, so it pass through the filter.
		univs = append(univs, newV)
	}
	return univs
}
