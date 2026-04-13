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

package remote

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	datastore "github.com/arcology-network/state-engine/storage/execstorage/livebackend"
)

type ReadonlyClient struct {
	addr       string
	path       string
	version    uint64 // State version
	localStore *datastore.LiveStorage
}

func NewReadonlyClient(addr string, path string, args ...any) *ReadonlyClient {
	readonlyClient := &ReadonlyClient{
		addr: addr,
		path: path,
	}

	if len(args) > 0 && args[0] != nil {
		readonlyClient.localStore = args[0].(*datastore.LiveStorage)
	}
	return readonlyClient
}

// func (this *ReadonlyClient) GetStateVersion() uint64        { return this.version }
// func (this *ReadonlyClient) SetStateVersion(version uint64) { this.version = version }

// Get from the server connected
func (this *ReadonlyClient) Get(key string) ([]byte, error) {
	if this.localStore != nil {
		v, err := this.localStore.Retrieve(key, nil)
		if err != nil {
			return []byte{}, err
		}
		return this.localStore.Encoder(nil)(key, v)
	} else {
		base, err := url.Parse(this.addr)
		if err != nil {
			return nil, errors.New("State-engine ReadonlyClient Error: The website is unreachable !")
		}

		base.Path = this.path
		params := url.Values{}
		params.Add("key", key)
		base.RawQuery = params.Encode()

		resp, err := http.Get(base.String())
		if err != nil {
			return nil, err
		}

		bytes, err := io.ReadAll(resp.Body)
		if err == nil {
			return bytes, nil
		}
		return nil, err
	}
}

// Get fromt the server connected
func (this *ReadonlyClient) GetBatch(keys []string) ([][]byte, error) {
	if this.localStore != nil {
		results := make([][]byte, len(keys))
		for i := 0; i < len(keys); i++ {
			results[i], _ = this.Get(keys[i])
		}
		return results, nil

	} else {
		// Get from the server
		return nil, fmt.Errorf("REMOTE READ NOT IMPLEMENTED")
	}
}

// Ready only, do nothing
func (*ReadonlyClient) Set(path string, v []byte) error           { return nil }
func (*ReadonlyClient) SetBatch(paths []string, v [][]byte) error { return nil }
func (*ReadonlyClient) Query(pattern string, condition func(string, []byte) bool) ([]string, [][]byte, error) {
	return []string{}, [][]byte{}, nil
}
