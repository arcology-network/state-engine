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
package common

import (
	"encoding/hex"
	"errors"
	"reflect"

	commutative "github.com/arcology-network/common-lib/crdt/commutative"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	"github.com/arcology-network/common-lib/crdt/statecell"
	evmcommon "github.com/ethereum/go-ethereum/common"
)

// Parse the address and selector from the given key.
// key: "blcc://eth1.0/account/[0x41eff1c3adfca1ccacced2198241747863dbf800]/profiles/[0x12345678/]" => return "0x41eff1c3adfca1ccacced2198241747863dbf800", "0x12345678"
func NewPathBuilderFromPath(path string) (*PathBuilder, error) {
	addr, selector, err := ParseAddressAndSelector(path)
	return &PathBuilder{Address: addr, Selector: selector, Platform: ETH_PATH}, err
}

// Parse the address and selector from the given strings.
func ParseAddressAndSelector(path string) (evmcommon.Address, [4]byte, error) {
	acct, selector := "", ""
	if len(path) >= ETH_ACCOUNT_FULL_LENGTH {
		acct = path[ETH_ACCOUNT_PREFIX_LENGTH:ETH_ACCOUNT_FULL_LENGTH]
	}

	if len(path) >= ETH_ACCOUNT_FULL_LENGTH+len(PATH_FUNC_PROFILE)+SELECTOR_LENGTH {
		selector = path[ETH_ACCOUNT_FULL_LENGTH+len(PATH_FUNC_PROFILE) : ETH_ACCOUNT_FULL_LENGTH+len(PATH_FUNC_PROFILE)+SELECTOR_LENGTH]
	}

	Address, Selector := evmcommon.Address{}, [4]byte{}
	addrBytes, err := hex.DecodeString(acct)
	if err != nil {
		return Address, Selector, errors.New("Invalid eth account address: " + acct)
	}
	copy(Address[:], addrBytes)

	selectorBytes, err := hex.DecodeString(selector)
	if err != nil {
		return Address, Selector, errors.New("Invalid eth account selector: " + selector)
	}
	copy(Selector[:], selectorBytes)
	return Address, Selector, nil
}

// CreateDefaultPaths creates default paths for an account in the storage committer.
func CreateDefaultPaths(tx uint64, acct string, store interface {
	IfExists(string) bool
	Write(uint64, string, any, ...any) (int64, error)
}) ([]*statecell.StateCell, error) {

	paths, typeids := NewPlatform().GetDefault(acct)

	transitions := []*statecell.StateCell{}
	for i, path := range paths {
		var v any
		switch typeids[i] {
		case commutative.PATH: // Path
			v = commutative.NewPath()

		case uint8(reflect.Kind(noncommutative.STRING)): // delta big int
			v = noncommutative.NewString("")

		case uint8(reflect.Kind(commutative.UINT256)): // delta big int
			v = commutative.NewUnboundedU256()

		case uint8(reflect.Kind(commutative.UINT64)):
			v = commutative.NewUnboundedUint64()

		case uint8(reflect.Kind(noncommutative.INT64)):
			v = new(noncommutative.Int64)

		case uint8(reflect.Kind(noncommutative.BYTES)):
			v = noncommutative.NewBytes([]byte{})
		}

		// fmt.Println(path)
		if !store.IfExists(path) {
			transitions = append(transitions, statecell.NewStateCell(tx, path, 0, 1, 0, v, nil))
			if _, err := store.Write(tx, path, v); err != nil { // root path
				return nil, err
			}
		}
	}
	return transitions, nil
}
