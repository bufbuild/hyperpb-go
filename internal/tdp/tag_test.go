package tdp

import (
	"google.golang.org/protobuf/encoding/protowire"
	"testing"
)

func TestTagOverflows(t *testing.T) {
	tag := EncodeTag(protowire.MaxValidNumber, protowire.BytesType)
	if tag.Overflows() {
		t.Error("protowire.MaxValidNumber should not overflow")
	}
}
