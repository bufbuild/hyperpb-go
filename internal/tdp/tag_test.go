package tdp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protowire"

	"buf.build/go/hyperpb/internal/tdp"
)

func TestTagOverflows(t *testing.T) {
	t.Parallel()
	tag := tdp.EncodeTag(protowire.MaxValidNumber, protowire.BytesType)
	assert.False(t, tag.Overflows(), "protowire.MaxValidNumber should not overflow")

	tag = tdp.EncodeTag(protowire.MaxValidNumber+1, protowire.BytesType)
	assert.True(t, tag.Overflows(), "protowire.MaxValidNumber+1 should overflow")
}
