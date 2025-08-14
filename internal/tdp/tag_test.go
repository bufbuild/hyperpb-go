package tdp

import (
	"testing"
	
	"google.golang.org/protobuf/encoding/protowire"
)

func TestTagOverflows(t *testing.T) {
	tag := EncodeTag(protowire.MaxValidNumber, protowire.BytesType)
	if tag.Overflows() {
		t.Error("protowire.MaxValidNumber should not overflow")
	}

	tag = EncodeTag(protowire.MaxValidNumber+1, protowire.BytesType)
	if !tag.Overflows() {
		t.Error("protowire.MaxValidNumber+1 should overflow")
	}
}
