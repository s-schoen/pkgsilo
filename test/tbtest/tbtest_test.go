package tbtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_Passes(t *testing.T) {
	t.Parallel()

	rec := Run(t, func(*Recorder) {})

	assert.False(t, rec.Failed())
	assert.Empty(t, rec.Messages())
}

func TestRun_RecordsErrors(t *testing.T) {
	t.Parallel()

	rec := Run(t, func(tb *Recorder) {
		tb.Errorf("first %d", 1)
		tb.Error("second")
	})

	assert.True(t, rec.Failed())
	assert.Equal(t, "first 1\nsecond", rec.Messages())
}

func TestRun_FailNowStopsFn(t *testing.T) {
	t.Parallel()
	reached := false

	rec := Run(t, func(tb *Recorder) {
		tb.Fatal("stop")
		reached = true
	})

	assert.True(t, rec.Failed())
	assert.Equal(t, "stop", rec.Messages())
	assert.False(t, reached, "fn must stop at Fatal")
}
