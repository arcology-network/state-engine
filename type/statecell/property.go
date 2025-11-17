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
	"unsafe"

	"github.com/arcology-network/common-lib/common"
	"github.com/cespare/xxhash"
)

type Property struct {
	vType         uint8
	tx            uint64  // Transaction ID
	generation    uint64  // Generation ID
	sequence      uint64  // Sequence ID
	path          *string // Key
	pathBytes     []byte  // for fast path comparison in sorin
	keyHash       uint64  // keyHash of the key, for faster comparison
	reads         uint32  // The number of reads
	writes        uint32  // The number of writes
	deltaWrites   uint32  // The number of delta writes
	sizeInStorage uint64  // Size in storage, which is guaranteed to be isCommitted already.
	gasUsed       uint64  // Gas used up to this point.
	msg           string

	ifSkipConflictCheck bool // If affected by conflict status or not.
	isExpanded          bool // If true, it is used in the local cache.
	isBlockBound        bool // If true, it is not persisted to the storage.
	isCommitted         bool // If the key exists in the source, which can be a cache or a storage.
	isDeleted           bool // If the value is deleted. Without this the conflict detection will mixed deletes up with normal wirtes whose values are removed for serialization speed.
	IsInConflict        bool // This is for the conflict detection module

	reclaimFunc func(any)
}

func NewProperty(tx uint64, key string, reads, writes uint32, deltaWrites uint32, vType uint8, ifSkipConflictCheck, isCommitted bool) *Property {
	return &Property{
		vType:               vType,
		sizeInStorage:       0,
		ifSkipConflictCheck: ifSkipConflictCheck,
		tx:                  tx,

		path:        &key,
		pathBytes:   unsafe.Slice(unsafe.StringData(key), len(key)),
		keyHash:     xxhash.Sum64String(key),
		reads:       reads,
		writes:      writes,
		deltaWrites: deltaWrites,
		isDeleted:   false,
	}
}

func (this *Property) Reset() {
	this.vType = 0
	this.sizeInStorage = 0
	this.ifSkipConflictCheck = false
	this.tx = 0
	this.generation = 0
	this.sequence = 0
	this.path = nil
	this.keyHash = 0
	this.reads = 0
	this.writes = 0
	this.deltaWrites = 0
	this.gasUsed = 0
	this.isDeleted = false
	this.reclaimFunc = nil
}

func (this *Property) Merge(other *Property) bool {
	this.reads += other.reads
	this.writes += other.writes
	this.deltaWrites += other.deltaWrites
	this.gasUsed += other.gasUsed
	this.sizeInStorage = common.Max(this.sizeInStorage, other.sizeInStorage)
	this.ifSkipConflictCheck = this.ifSkipConflictCheck || other.ifSkipConflictCheck
	return this.keyHash == other.keyHash
}

func (this *Property) IsBlockBound() bool              { return this.isBlockBound } // If true, it is not persisted to the storage.
func (this *Property) SetBlockBound(isBlockBound bool) { this.isBlockBound = isBlockBound }

func (this *Property) IsExpanded() bool            { return this.isExpanded } // Substituted by a wildcard path.
func (this *Property) SetExpanded(isExpanded bool) { this.isExpanded = isExpanded }

func (this *Property) SizeInStorage() uint64 { return this.sizeInStorage }
func (this *Property) GetMsg() string        { return this.msg }
func (this *Property) SetMsg(msg string)     { this.msg = msg }
func (this *Property) AppendMsg(msg string)  { this.msg = this.msg + "\n" + msg }

func (this *Property) IfSkipConflictCheck() bool { return this.ifSkipConflictCheck }
func (this *Property) SkipConflictCheck(v bool)  { this.ifSkipConflictCheck = v }

func (this *Property) GetTx() uint64     { return this.tx }
func (this *Property) SetTx(txId uint64) { this.tx = txId }

func (this *Property) GetGeneration() uint64 { return this.generation }
func (this *Property) GetSequence() uint64   { return this.sequence }

func (this *Property) SetGeneration(id uint64) { this.generation = id }
func (this *Property) SetSequence(id uint64)   { this.sequence = id }

func (this *Property) GetPath() *string     { return this.path }
func (this *Property) SetPath(path *string) { this.path = path }
func (this *Property) ClearPath()           { *this.path = (*this.path)[:0] }

func (this *Property) TypeID() uint8 { return this.vType }

func (this *Property) Reads() uint32       { return this.reads }
func (this *Property) Writes() uint32      { return this.writes } // Exist in cache as a failed read
func (this *Property) DeltaWrites() uint32 { return this.deltaWrites }

func (this *Property) IncrementReads(reads uint32)             { this.reads += reads }
func (this *Property) IncrementWrites(writes uint32)           { this.writes += writes }
func (this *Property) IncrementDeltaWrites(deltaWrites uint32) { this.deltaWrites += deltaWrites }

func (this *Property) IsReadOnly() bool   { return this.Writes() == 0 && this.DeltaWrites() == 0 }
func (this *Property) IsCommitted() bool  { return this.isCommitted } // Exist in cache as a failed read
func (this *Property) SetPreexist(v bool) { this.isCommitted = v }    // Exist in cache as a failed read

// func (this *Property) Persistent() bool { return this.ifSkipConflictCheck }

// This is for debugging purposes only, do not use it in production code!!!
func (this *Property) SetIsDeleted(flag bool) { this.isDeleted = flag }

// Check if the key exists in the source, which can be a cache or a storage，which isn't guaranteed
// to be the same as the cache. It is possible that the key exists in the cache but not in the storage.
// This means that the key is a new key that hasn't been isCommitted to the storage yet.
func (this *Property) IsCommiitted(key string, source any) bool {
	return source.(interface{ IfExists(string) bool }).IfExists(key)
}

func (this *Property) Equal(other *Property) bool {
	return this.vType == other.vType &&
		this.tx == other.tx &&
		this.generation == other.generation &&
		this.sequence == other.sequence &&
		this.keyHash == other.keyHash && // compare the key hashes first for faster comparison.
		*this.path == *other.path &&
		this.reads == other.reads &&
		this.writes == other.writes &&
		this.deltaWrites == other.deltaWrites &&
		this.isDeleted == other.isDeleted &&
		this.msg == other.msg
}

func (this *Property) Clone() Property {
	return Property{
		vType:         this.vType,
		tx:            this.tx,
		generation:    this.generation,
		sequence:      this.sequence,
		path:          this.path,
		keyHash:       this.keyHash,
		reads:         this.reads,
		deltaWrites:   this.deltaWrites,
		writes:        this.writes,
		isDeleted:     this.isDeleted,
		gasUsed:       this.gasUsed,
		sizeInStorage: this.sizeInStorage,
		isCommitted:   this.isCommitted,
		reclaimFunc:   this.reclaimFunc,
		msg:           this.msg,
	}
}
