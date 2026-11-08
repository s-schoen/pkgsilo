package apitest

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	"pkgsilo/test/tbtest"
)

func TestAssertReadableMembers(t *testing.T) {
	t.Parallel()
	type masked struct {
		ID   string                 `json:"id"`
		Name envelope.Field[string] `json:"name,omitzero"`
	}
	type plain struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type withoutOmitzero struct {
		ID   string                 `json:"id"`
		Name envelope.Field[string] `json:"name"`
	}
	tests := []struct {
		name        string
		body        reflect.Type
		readable    []string
		wantMessage string
	}{
		{name: "masked", body: reflect.TypeFor[masked](), readable: []string{"name"}},
		{
			name: "plain member", body: reflect.TypeFor[plain](), readable: []string{"name"},
			wantMessage: "not an envelope.Field",
		},
		{
			name: "without omitzero", body: reflect.TypeFor[withoutOmitzero](), readable: []string{"name"},
			wantMessage: "lacks the omitzero option",
		},
		{name: "member without action", body: reflect.TypeFor[masked](), wantMessage: "must be the readable fields"},
		{
			name: "action without member", body: reflect.TypeFor[masked](), readable: []string{"name", "email"},
			wantMessage: "must be the readable fields",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := tbtest.Run(t, func(tb *tbtest.Recorder) { AssertReadableMembers(tb, tt.body, tt.readable) })

			assert.Equal(t, tt.wantMessage != "", rec.Failed(), rec.Messages())
			assert.Contains(t, rec.Messages(), tt.wantMessage)
		})
	}
}
