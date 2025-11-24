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

import "testing"

func TestGetSegmentAt(t *testing.T) {
	tests := []struct {
		path     string
		n        int
		leading  bool
		trailing bool
		expected string
		ok       bool
	}{
		// ───────────────────────────────────────────────
		// Basic paths
		// ───────────────────────────────────────────────
		{"/a/b/c", 0, false, false, "a", true},
		{"/a/b/c", 1, false, false, "b", true},
		{"/a/b/c", 2, false, false, "c", true},

		// Leading slash included
		{"/a/b/c", 1, true, false, "/b", true},

		// Trailing slash included
		{"/a/b/c", 1, false, true, "b/", true},

		// Both
		{"/a/b/c", 1, true, true, "/b/", true},

		// Out of range
		{"/a/b/c", 3, false, false, "", false},

		// ───────────────────────────────────────────────
		// Consecutive slashes (skip empty segments)
		// ───────────────────────────────────────────────
		{"a//b///c", 0, false, false, "a", true},
		{"a//b///c", 1, false, false, "b", true},
		{"a//b///c", 2, false, false, "c", true},
		{"a//b///c", 2, true, false, "/c", true},
		{"a//b///c", 2, false, true, "c/", true},
		{"a//b///c", 2, true, true, "/c/", true},

		// Out of range
		{"a//b///c", 3, false, false, "", false},

		// ───────────────────────────────────────────────
		// No leading slash
		// ───────────────────────────────────────────────
		{"a/b/c", 0, false, false, "a", true},
		{"a/b/c", 1, false, false, "b", true},
		{"a/b/c", 2, false, false, "c", true},
		{"a/b/c", 0, true, false, "/a", true},
		{"a/b/c", 2, false, true, "c/", true},

		// ───────────────────────────────────────────────
		// Trailing slash shouldn't create extra segment
		// ───────────────────────────────────────────────
		{"a/b/c/", 0, false, false, "a", true},
		{"a/b/c/", 1, false, false, "b", true},
		{"a/b/c/", 2, false, false, "c", true},
		{"a/b/c/", 3, false, false, "", false},

		// Leading AND trailing
		{"a/b/c/", 2, true, true, "/c/", true},

		// ───────────────────────────────────────────────
		// Real ETH-style path with :// and empty segment
		// ───────────────────────────────────────────────
		{"blcc://eth1.0/account/0xabc/storage/container/1", 0, false, false, "blcc:", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 1, false, false, "eth1.0", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 2, false, false, "account", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 3, false, false, "0xabc", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 4, false, false, "storage", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 5, false, false, "container", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 6, false, false, "1", true},

		// With leading/trailing options on ETH path
		{"blcc://eth1.0/account/0xabc/storage/container/1", 2, true, false, "/account", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 2, false, true, "account/", true},
		{"blcc://eth1.0/account/0xabc/storage/container/1", 2, true, true, "/account/", true},

		// Out of range
		{"blcc://eth1.0/account/0xabc/storage/container/1", 7, false, false, "", false},
	}

	for _, tt := range tests {
		got, ok := GetSegmentAt(tt.path, tt.n, tt.leading, tt.trailing)
		if ok != tt.ok || got != tt.expected {
			t.Errorf("GetSegmentAt(%q, %d, %v, %v) = (%q,%v), want (%q,%v)",
				tt.path, tt.n, tt.leading, tt.trailing,
				got, ok, tt.expected, tt.ok)
		}
	}
}
