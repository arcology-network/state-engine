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

	evmcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type PathBuilder struct {
	Address  evmcommon.Address
	Selector [4]byte
	Platform uint8
}

// Create a new PathBuilder with the specified platform.
func NewPathBuilder(platform uint8) *PathBuilder {
	return &PathBuilder{
		Platform: platform,
	}
}

// Create a new PathBuilder for Ethereum platform.
func NewEthPathBuilder() *PathBuilder {
	return &PathBuilder{
		Platform: ETH_PATH,
	}
}

// Parse the address and selector from the given key.
// key: "blcc://eth1.0/account/[0x41eff1c3adfca1ccacced2198241747863dbf800]/profiles/[0x12345678/]" => return "0x41eff1c3adfca1ccacced2198241747863dbf800", "0x12345678"
func NewPathBuilderFromPath(path string) (*PathBuilder, error) {
	addr, selector, err := ParseAddressFromPath(path)
	return &PathBuilder{Address: addr, Selector: selector, Platform: ETH_PATH}, err
}

// Parse the address and selector from the given strings.
func ParseAddressFromPath(path string) (evmcommon.Address, [4]byte, error) {
	acct, selector := "", ""
	if len(path) >= ETH_ACCOUNT_FULL_LENGTH {
		acct = path[ETH_ACCOUNT_PREFIX_LENGTH:ETH_ACCOUNT_FULL_LENGTH]
	}

	if len(path) >= ETH_ACCOUNT_FULL_LENGTH+len(FUNC_PROFILE_PATH)+SELECTOR_LENGTH {
		selector = path[ETH_ACCOUNT_FULL_LENGTH+len(FUNC_PROFILE_PATH) : ETH_ACCOUNT_FULL_LENGTH+len(FUNC_PROFILE_PATH)+SELECTOR_LENGTH]
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

// Build the subpath under the callee profile.
// e.g., blcc://eth1.0/account/[0x41eff1c3adfca1ccacced2198241747863dbf800]/profiles/[0x12345678]/paraDegree
func (this *PathBuilder) ProfileField(subpath string) string {
	return ETH_ACCOUNT_PREFIX + hexutil.Encode(this.Address[:]) +
		FUNC_PROFILE_PATH + hexutil.Encode(this.Selector[:]) + "/" + subpath
}
