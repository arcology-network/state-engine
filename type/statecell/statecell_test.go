/*
 *   Copyright (c) 2024 Arcology Network

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

package statecell

import (
	"fmt"
	"testing"

	"github.com/arcology-network/common-lib/exp/slice"
	"github.com/arcology-network/common-lib/exp/softdeltaset"
	stgcommon "github.com/arcology-network/storage-committer/common"
	commutative "github.com/arcology-network/storage-committer/type/commutative"
	"github.com/holiman/uint256"
)

func AliceAccount() string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	// rand.Seed(1)
	b := make([]rune, 40)
	for i := range b {
		b[i] = letters[1]
	}
	return string(b)
}

func TestUnivalueCodecUint64(t *testing.T) {
	/* Commutative Int64 Test */
	alice := AliceAccount()

	u64 := commutative.NewBoundedUint64(0, 100)
	in := NewStateCell(1, "blcc://eth1.0/account/"+alice+"/storage/ctrn-0/elem-000", 3, 4, 0, u64, nil)
	in.reads = 1
	in.writes = 2
	in.deltaWrites = 3
	in.isCommitted = true

	bytes := in.Encode()
	v := (&StateCell{}).Decode(bytes).(*StateCell)

	property := v.Property
	inProperty := in.Property
	if !(inProperty).Equal(&property) {
		t.Error("Error")
	}

	out := v.Value()

	if !(in.value.(*commutative.Uint64)).Equal(out.(*commutative.Uint64)) {
		t.Error("Error")
	}
}

func TestUnivalueCodecU256(t *testing.T) {
	alice := AliceAccount() /* Commutative Int64 Test */
	u256 := commutative.NewBoundedU256(uint256.NewInt(0), uint256.NewInt(100))

	in := NewStateCell(1, "blcc://eth1.0/account/"+alice+"/storage/ctrn-0/elem-000", 3, 4, 0, u256, nil)
	in.reads = 1
	in.writes = 2
	in.deltaWrites = 3

	bytes := in.Encode()
	v := (&StateCell{}).Decode(bytes).(*StateCell)
	out := v.Value()

	raw := in.Value().(*commutative.U256).Value().(uint256.Int)

	outV := out.(*commutative.U256).Value().(uint256.Int)
	dv, _ := in.Value().(*commutative.U256).Delta()
	deltaV := dv.(uint256.Int)

	odv, _ := out.(*commutative.U256).Delta()
	otherv := odv.(uint256.Int)

	flag := (&deltaV).Cmp(&(otherv)) != 0
	if raw.Cmp((*uint256.Int)(&outV)) != 0 || flag {
		t.Error("Error")
	}

	if in.vType != v.vType ||
		in.tx != v.tx ||
		*in.path != *v.path ||
		in.writes != v.writes ||
		in.deltaWrites != v.deltaWrites ||
		in.isCommitted != v.isCommitted || in.msg != v.msg {
		t.Error("Error: mismatch after decoding")
	}

	str1 := "First string"
	str2 := "Second string"

	result := str1 + "\n" + str2

	fmt.Println(result)
}

func TestUnivalueCodeMeta(t *testing.T) {
	/* Commutative Int64 Test */
	alice := AliceAccount()

	meta := commutative.NewPath()
	meta.(*commutative.Path).SetSubPaths([]string{"e-01", "e-001", "e-002", "e-002"})
	meta.(*commutative.Path).SetAdded([]string{"+01", "+001", "+002", "+002"})
	meta.(*commutative.Path).InsertRemoved([]string{"-091", "-0092", "-092", "-092", "-097"})

	in := NewStateCell(1, "blcc://eth1.0/account/"+alice+"/storage/ctrn-0/elem-000", 3, 4, 11, meta, nil)
	in.reads = 1
	in.writes = 2
	in.deltaWrites = 3

	inKeys, _, _ := in.Value().(stgcommon.Type).Get()

	bytes := in.Encode()
	out := (&StateCell{}).Decode(bytes).(*StateCell)
	outKeys, _, _ := out.Value().(stgcommon.Type).Get()

	if !slice.EqualSet(inKeys.(*softdeltaset.DeltaSet[string]).Elements(), outKeys.(*softdeltaset.DeltaSet[string]).Elements()) {
		t.Error("Error")
	}
}

func TestPropertyCodecUint64(t *testing.T) {
	/* Commutative Int64 Test */
	alice := AliceAccount()

	u256 := commutative.NewBoundedUint64(0, 100).(*commutative.Uint64)
	in := NewProperty(1, "blcc://eth1.0/account/"+alice+"/storage/ctrn-0/elem-000", 3, 4, 0, u256.TypeID(), true, false)
	in.reads = 1
	in.writes = 2
	in.deltaWrites = 3
	in.msg = "hello"

	bytes := in.Encode()
	out := (&Property{}).Decode(bytes).(*Property)

	if !in.Equal(out) {
		t.Error("Error")
	}
}

// func BenchmarkAccountMerkleImportPerf(t *testing.B) {
// 	data := [][]byte{}
// 	for i := 0; i < 1000000; i++ {
// 		v := sha256.Sum256([]byte(fmt.Sprint(i)))
// 		data = append(data, v[:])
// 	}

// 	t0 := time.Now()
// 	s1 := codec.Byteset(data).Encode()
// 	codec.Byteset{}.Decode(s1)
// 	fmt.Println("Code.Byteset: ", time.Since(t0), len(s1))

// 	t0 = time.Now()
// 	s2 := ethrlp.Bytes{}.Encode(data)
// 	_, err := ethrlp.Bytes{}.Decode(s2)

// 	rlp.EncodeToBytes(s2)
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	fmt.Println("ethrlp.Bytes{}.Encode: ", time.Since(t0), len(s2), float64(len(s1))/float64(len(s2)))
// }
