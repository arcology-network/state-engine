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
	"github.com/arcology-network/common-lib/codec"
	evmcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type PathBuilder struct {
	Sender   evmcommon.Address
	Address  evmcommon.Address
	Selector [4]byte
	Platform uint8
}

// Create a new PathBuilder with the specified platform.
func NewContainerPathBuilder(platform uint8) *PathBuilder {
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

// Derive a unique identifier (UID) from the address and selector.
func (this *PathBuilder) DeriveUID() uint64 {
	return DeriveEthCalleeUID(this.Address, this.Selector)
}

func (this *PathBuilder) DeriveUIDFromPath(path string) uint64 {
	addr, selector, err := ParseAddressAndSelector(path)
	if err != nil {
		return 0
	}

	this.Address = addr
	this.Selector = selector
	return this.DeriveUID()
}

// Build the subpath under the callee profile.
// e.g., blcc://eth1.0/account/[0x41eff1c3adfca1ccacced2198241747863dbf800]/profiles/[0x12345678]/paraDegree
func (this *PathBuilder) ProfileField(field string) string {
	return ETH_ACCOUNT_PREFIX + hexutil.Encode(this.Address[:]) +
		PATH_FUNC_PROFILE + hexutil.Encode(this.Selector[:]) + "/" + field
}

// The Path for the callee profile field.
// e.g., blcc://eth1.0/account/[0x41eff1c3adfca1ccacced2198241747863dbf800]/profiles/[0x12345678]/paraDegree
func (this *PathBuilder) UnderSenderPath(subpath string) string {
	return ETH_ACCOUNT_PREFIX + hexutil.Encode(this.Sender[:]) + subpath
}

// Derive a unique identifier (UID) from the address and selector.
func DeriveEthCalleeUID(addr [20]byte, selector [4]byte) uint64 {
	return uint64(codec.Uint64(0).FromBytes(append(
		addr[:SHORT_CONTRACT_ADDRESS_LENGTH], selector[:]...)))
}
