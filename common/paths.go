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

package common

import (
	"strings"

	common "github.com/arcology-network/common-lib/common"
	commutative "github.com/arcology-network/common-lib/crdt/commutative"
	noncommutative "github.com/arcology-network/common-lib/crdt/noncommutative"
	mapi "github.com/arcology-network/common-lib/exp/map"
	"github.com/arcology-network/common-lib/exp/slice"
)

type Platform struct {
	syspaths map[string]uint8
}

// Returns a list of paths that need to be created under the account automatically when the account is created.
func NewPlatform() *Platform {
	return &Platform{
		map[string]uint8{
			"/":        commutative.PATH,
			"/code":    noncommutative.BYTES,
			"/nonce":   commutative.UINT64,
			"/balance": commutative.UINT256,

			// Arcology specific paths
			FUNC_PROFILE_PATH:     commutative.PATH,
			"/storage/":           commutative.PATH,
			"/storage/container/": commutative.PATH, // Container storage
			"/storage/native/":    commutative.PATH, // Native storage
		},
	}
}

func ETHAccountShard(numOfShard int, key string) int {
	if len(key) < 24 {
		panic("Invalid eth1.0 account shard key: " + key)
	}
	return (common.Hex2int(key[22])*16 + common.Hex2int(key[23])) % numOfShard
}

// Get ths builtin paths
func (this *Platform) GetDefault(acct string) ([]string, []uint8) {
	paths, typeIds := mapi.KVs(this.syspaths)
	slice.SortBy1st(paths, typeIds, func(lhv, rhv string) bool { return lhv < rhv })

	for i, path := range paths {
		paths[i] = ETH_ACCOUNT_PREFIX + acct + path
	}
	return paths, typeIds
}

// These paths won't keep the sub elements
func (this *Platform) IsSysPath(path string) bool {
	if len(path) <= ETH_ACCOUNT_FULL_LENGTH {
		return true
	}

	subPath := path[ETH_ACCOUNT_FULL_LENGTH:] // Removed the shared prefix part
	_, ok := this.syspaths[subPath]
	return ok
}

// A system path and an child of the system paths as well.
func (this *Platform) IsImmediateChildOfSysPath(path string) bool {
	if this.IsSysPath(path) {
		return true
	}

	parent, _ := GetParentPath(path)
	if this.IsContainerPath(parent) { // Still need to keep track of the elements under the container path.
		return false
	}

	return this.IsSysPath(parent) ||
		!strings.Contains(parent, "/") // All but the root has "/", root is also a system path.
}

// If the path of a concurrent container, it is a concurrent path.
func (*Platform) IsContainerPath(path string) bool {
	return strings.HasSuffix(path, "/container/")
}

func ParseAccountAddr(acct string) (string, string, string) {
	if len(acct) < ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH {
		return acct, "", ""
	}
	return acct[:ETH_ACCOUNT_PREFIX_LENGTH],
		acct[ETH_ACCOUNT_PREFIX_LENGTH : ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH],
		acct[ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH:]
}

func GetAccountAddr(acct string) string {
	if len(acct) < ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH {
		return acct
	}
	return acct[ETH_ACCOUNT_PREFIX_LENGTH : ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH]
}

func GetPathUnder(key, prefix string) string {
	if len(key) > ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH {
		subKey := key[ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH:]
		if subKey != prefix && strings.HasPrefix(subKey, prefix) {
			return subKey[len(prefix):]
		}
	}
	return ""
}

// IsEthPath checks if the path is an eth path, some paths are not Arcology only.
func IsEthPath(path string) bool {
	return !strings.HasSuffix(path, "container/")
}
