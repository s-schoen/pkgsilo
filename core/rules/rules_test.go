package rules

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
)

func TestTextRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rule validation.Rule
		text string
		// wantErr is the message of the validation error, empty for valid text.
		wantErr string
	}{
		{name: "not blank", rule: NotBlank, text: " Ada "},
		{name: "not blank empty", rule: NotBlank, text: ""},
		{name: "not blank spaces", rule: NotBlank, text: "   ", wantErr: "cannot be blank"},
		{name: "not blank other whitespace", rule: NotBlank, text: "\t\n 　", wantErr: "cannot be blank"},
		{name: "no control characters", rule: NoControlCharacters, text: "Ada Lovelace (née Byron) 🙂"},
		{name: "no control characters empty", rule: NoControlCharacters, text: ""},
		{
			name: "no control characters NUL", rule: NoControlCharacters, text: "Ada\x00",
			wantErr: "must not contain control characters",
		},
		{
			name: "no control characters line break", rule: NoControlCharacters, text: "Ada\nLovelace",
			wantErr: "must not contain control characters",
		},
		{
			name: "no control characters C1", rule: NoControlCharacters, text: "Ada\u0085",
			wantErr: "must not contain control characters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validation.Validate(tt.text, tt.rule)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			// A validation.Error, which the API and the tools report as an invalid member, not as a bug.
			var valueErr validation.Error
			if assert.ErrorAs(t, err, &valueErr) {
				assert.Equal(t, tt.wantErr, valueErr.Error())
			}
		})
	}
}
