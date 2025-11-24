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
	"math"
)

const (
	MAX_DEPTH uint8 = 12
	SYSTEM          = math.MaxInt32

	UNKNOWN  uint8 = iota // Unknown path type
	ETH_PATH uint8 = 1    // Ethereum path type
	ARN_PATH uint8 = 2    // Arcology path type

	MIN_READ_SIZE  uint64 = 32 // A single read request should at least cost
	MIN_WRITE_SIZE uint64 = 32 // A single write request should at least cost
)

var WARN_OUT_OF_LOWER_LIMIT string = "Warning: Out of the lower limit!"
var WARN_OUT_OF_UPPER_LIMIT string = "Warning: Out of the upper limit!"
var WARN_ACCESS_CONFLICT = "Warning: State access conflict detected!"
var WARN_EXEC_FAILED = "Warning: Execution execution failed!"

// Path constants for storage paths
const (
	ETH                = "blcc://eth1.0/"
	ETH_ACCOUNT_PREFIX = ETH + "account/"
	// GAS_PREPAYERS                      = ETH + "prepayers/" // Gas prepayment for the deferred execution.
	ETH_ACCOUNT_PREFIX_LENGTH = len(ETH_ACCOUNT_PREFIX)
	ETH_ACCOUNT_LENGTH        = 42 // 40 hex digits + 0x
	ETH_ACCOUNT_FULL_LENGTH   = ETH_ACCOUNT_PREFIX_LENGTH + ETH_ACCOUNT_LENGTH

	//e.g.  blcc://eth1.0/account/0x41eff1c3adfca1ccacced2198241747863dbf800/storage/
	ETH_STORAGE_PREFIX               = ETH_ACCOUNT_PREFIX + "storage/"
	ETH_STORAGE_PREFIX_LENGTH        = len(ETH_STORAGE_PREFIX) + ETH_ACCOUNT_LENGTH
	ETH_STORAGE_NATIVE_PREFIX_LENGTH = ETH_STORAGE_PREFIX_LENGTH + len(ETH_NATIVE_PATH)

	SHORT_CONTRACT_ADDRESS_LENGTH = 4                                               // First 4 bytes for address
	SELECTOR_LENGTH               = 4                                               // 4 bytes for signature
	CALLEE_ID_LENGTH              = SHORT_CONTRACT_ADDRESS_LENGTH + SELECTOR_LENGTH // Can be represented as uint64

	// If the path starts with the following prefixes, it means the path should be persisted to Ethereum storage.
	ETH_NATIVE_PATH = "/native/"

	// PATH_ROOT is the base namespace for all account-level state entries.
	PATH_ROOT = "/"

	// PATH_CODE maps to the contract bytecode stored for an account.
	PATH_CODE = "/code"

	// PATH_NONCE maps to the transaction nonce associated with an account.
	PATH_NONCE = "/nonce"

	// PATH_BALANCE maps to the native token balance of an account.
	PATH_BALANCE = "/balance"

	// PATH_FUNC_PROFILE is the namespace for per-function execution metadata:
	// parallelism degree, deferred-execution prepayment, conflict sets, etc.
	PATH_FUNC_PROFILE = FUNC_PROFILE_PATH

	// PATH_STORAGE_ROOT is the root namespace for a contract’s storage.
	PATH_STORAGE_ROOT = "/storage/"

	// PATH_ARCO_CONTAINER is the namespace for structured storage objects
	PATH_ARCO_CONTAINER = "container/"

	// PATH_ETH_NATIVE is the namespace for ETH primitive storage values
	PATH_ETH_NATIVE = "native/"

	// PATH_STORAGE_CONTAINER is the namespace for structured storage objects
	// (containers, CRDT-based types, concurrent arrays, ordered maps, etc.).
	PATH_STORAGE_CONTAINER = PATH_STORAGE_ROOT + PATH_ARCO_CONTAINER

	// PATH_STORAGE_NATIVE is the namespace for ETH primitive storage values
	PATH_STORAGE_NATIVE = PATH_STORAGE_ROOT + PATH_ETH_NATIVE

	// function property paths, that can be created on the fly.
	FUNC_PROFILE_PATH  = "/profiles/"
	PARALLELISM_DEGREE = "parallelism-degree" // The execution parallelism of the function, either parallel or sequential
	DEFERRED_PAYMENT   = "prepayment"         // Amount of gas prepaid required for the function's deferred execution
	PREPAYERS          = "prepayers/"         // Address of the gas prepayers for the function's deferred execution
	CONFLICT_INFO_PATH = "conflicts/"         // The history of conflicts for the function, used for debugging and analysis
)

// For scheduler conflict management
const (
	MAX_CONFLICT_RATIO = 0.5
	MAX_NUM_CONFLICTS  = 256 //Functions have greater conflicts will be flagged as sequential.
)
