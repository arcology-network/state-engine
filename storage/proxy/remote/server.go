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
	"fmt"
	"net/http"

	crdtcommon "github.com/arcology-network/common-lib/crdt/common"
	stgintf "github.com/arcology-network/common-lib/storage/interface"
)

type ReadonlyServer struct {
	addr      string
	dataStore stgintf.ReadOnlyStore[string, crdtcommon.CRDT]
	encoder   func(interface{}) []byte
	decoder   func([]byte) (interface{}, error)
}

func NewReadonlyServer(addr string, encoder func(interface{}) []byte, decoder func([]byte) (interface{}, error), dataStore stgintf.ReadOnlyStore[string, crdtcommon.CRDT]) *ReadonlyServer {
	return &ReadonlyServer{
		addr:      addr,
		dataStore: dataStore,
		encoder:   encoder,
		decoder:   decoder,
	}
}

func (this *ReadonlyServer) Get(path string) ([]byte, error) {
	val, err := this.dataStore.Get(path)
	if err != nil {
		return []byte{}, err
	}
	return this.encoder(val), nil
}

// Get fromt the server connected
func (this *ReadonlyServer) GetBatch(paths []string) ([][]byte, error) {
	bytes := make([][]byte, len(paths))
	for i, v := range paths {
		val, err := this.dataStore.Get(v)
		if err != nil {
			fmt.Printf("ReadonlyServer GetBatch err: %v k: %v\n", err, v)
			continue
		}
		bytes[i] = this.encoder(val)
	}
	return bytes, nil
}

func (this *ReadonlyServer) Receive(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		if err := request.ParseForm(); err == nil {
			key := request.FormValue("key")
			if v, _ := this.dataStore.Get(key); v != nil {
				if _, err := writer.Write(this.encoder(v)); err != nil {
					fmt.Printf("ReadonlyServer Receive err: %v k: %v\n", err, key)
				}
			}
		}
	}
}
