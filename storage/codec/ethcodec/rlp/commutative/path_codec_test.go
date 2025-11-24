/*
 *   Copyright (c) 2023 Arcology Network

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

package commutativecodec

import (
	"testing"

	"github.com/arcology-network/common-lib/crdt/commutative"
)

func TestPath(t *testing.T) {
	/* Noncommutative Path Test*/
	meta := commutative.NewPath()
	inPath := meta.(*commutative.Path)

	inPath.SetSubPaths([]string{"e-01", "e-001", "e-002", "e-002"})
	inPath.SetAdded([]string{"+01", "+001", "+002", "+002"})
	inPath.InsertRemoved([]string{"-091", "-0092", "-092", "-092", "-097"})

	buffer := (&Path{Path: *inPath}).Encode()
	outPath := new(Path).Decode(buffer)

	if !inPath.Equal(outPath) {
		t.Error("Error: Don't match!!", inPath, outPath)
	}
}
