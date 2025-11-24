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
	"strings"
)

// Package common provides helpers for platform-specific account parsing and manipulation.
// ParseAccountAddr splits an account identifier into its prefix, canonical Ethereum address, and trailing suffix,
// returning empty strings for the latter two segments if the identifier is too short.

// ParseAccountAddr splits an account identifier into its prefix, canonical Ethereum address, and trailing suffix.
func ParseAccountAddr(acct string) (string, string, string) {
	if len(acct) < ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH {
		return acct, "", ""
	}
	return acct[:ETH_ACCOUNT_PREFIX_LENGTH],
		acct[ETH_ACCOUNT_PREFIX_LENGTH : ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH],
		acct[ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH:]
}

// GetAccountAddr extracts the canonical Ethereum address from a path string.
func GetAccountAddr(acct string) string {
	_, addr, _ := ParseAccountAddr(acct)
	return addr
	// if len(acct) < ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH {
	// 	return acct
	// }
	// return acct[ETH_ACCOUNT_PREFIX_LENGTH : ETH_ACCOUNT_PREFIX_LENGTH+ETH_ACCOUNT_LENGTH]
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

// If the value represented by the path should be written to Ethereum native storage.
func ShouldPersistToEth(path string) bool {
	if len(path) <= ETH_ACCOUNT_FULL_LENGTH+1 { //account + `/`
		// Ignore paths that are too short. Because they are default paths that not meant to be stored.
		// All the subpaths carry the full path, so no need to store the default paths.
		// e.g., blcc://eth1.0/account/0x41eff1c3adfca1ccacced2198241747863dbf800/
		// will not be persisted to either ETH or ACL.
		return false
	}

	if strings.HasSuffix(path, PATH_NONCE) ||
		strings.HasSuffix(path, PATH_BALANCE) ||
		strings.HasSuffix(path, PATH_CODE) {
		return true
	}

	// Any other paths that do not end with `/` are paths that are added by Arcology
	// to keep track of the sub paths, this isn't necessarily an ETH native storage.
	if strings.HasSuffix(path, "/") {
		return false
	}

	sub, ok := GetSegmentAt(path, 5, false, true)
	return ok && sub == PATH_ETH_NATIVE
}

// If the value represented by the path should be written to Arcology storage.
func ShouldPersistToArco(path string) bool {
	return true // Always persist to Arcology's execution storage.
}

// GetParentPath returns the parent path of the given key.
// If the key is empty or the root ("/"), it returns the key itself.
// Otherwise, it returns the substring of the key up to the last occurrence of "/".
func GetParentPath(key string) (string, string) {
	if len(key) == 0 || key == "/" { //Root or empty
		return key, key
	}
	path := key[:strings.LastIndex(key[:len(key)-1], "/")+1]
	return path, key[len(path):]
}

// IsEthAddress checks if the given key represents an Ethereum address.
func IsEthAddress(path string) bool {
	if !strings.HasPrefix(path, "0x") {
		return false
	}
	return len(path) == ETH_ACCOUNT_LENGTH || (len(path) == ETH_ACCOUNT_LENGTH+1 && path[len(path)-1] == '/')
}

// GetSegmentAt retrieves the n-th segment from a path string.
// Segments are separated by '/' and empty segments are ignored.
// It returns the segment and true if found, otherwise an empty string and false.
func GetSegmentAt(path string, n int, includeLeading, includeTrailing bool) (string, bool) {
	seg := -1
	start := -1

	for i := 0; i <= len(path); i++ {
		end := (i == len(path) || path[i] == '/')

		if start == -1 {
			if !end {
				start = i
			}
		} else {
			if end {
				seg++
				if seg == n {
					s := start
					e := i

					// Extract the raw segment
					segment := path[s:e]

					// Apply flags: ALWAYS prepend/append slash
					if includeLeading {
						segment = "/" + segment
					}
					if includeTrailing {
						segment = segment + "/"
					}
					return segment, true
				}
				start = -1
			}
		}
	}

	return "", false
}
