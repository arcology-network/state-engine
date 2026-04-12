package ethrlp

import (
	"testing"

	crdtc "github.com/arcology-network/common-lib/crdt/commutative"
	crdtnc "github.com/arcology-network/common-lib/crdt/noncommutative"
)

func TestRlpCodecUint32(t *testing.T) {
	value := crdtnc.NewUint32(12345)

	encoded, err := (RlpCodec{}).Encode("", value)
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decoded := (RlpCodec{}).Decode("", encoded, nil).(*crdtnc.Uint32)
	if !decoded.Equal(value) {
		t.Fatal("decode mismatch")
	}
}

func TestRlpCodecIgnoresPrototype(t *testing.T) {
	value := crdtc.NewUnboundedUint64().(*crdtc.Uint64)
	value.SetValue(uint64(88))

	encoded, err := (RlpCodec{}).Encode("", value)
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decoded := (RlpCodec{}).Decode("", encoded, crdtnc.NewUint32(1)).(*crdtc.Uint64)
	if decoded.Value().(uint64) != 88 {
		t.Fatal("decode mismatch")
	}

	min, max := decoded.Limits()
	if min.(uint64) != 0 || max.(uint64) == 0 {
		t.Fatal("limits mismatch")
	}
}