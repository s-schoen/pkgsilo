// Package tbtest runs test helpers against a recording testing.TB, so that the tests of the helpers
// in test/ can check which failures the helpers report.
package tbtest

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// Recorder is a testing.TB that records the failures reported to it instead of failing the test.
// Everything else is passed to the test it was created for.
type Recorder struct {
	testing.TB

	failed   bool
	messages []string
}

// Run calls fn with a Recorder for t and returns the Recorder once fn returns or calls FailNow.
// fn runs in its own goroutine, so that FailNow can stop it without stopping t.
func Run(t *testing.T, fn func(tb *Recorder)) *Recorder {
	t.Helper()

	rec := &Recorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(rec)
	}()
	<-done

	return rec
}

// Failed reports whether a failure was reported.
func (r *Recorder) Failed() bool {
	return r.failed
}

// Messages returns the reported failure messages, one per line.
func (r *Recorder) Messages() string {
	return strings.Join(r.messages, "\n")
}

// Helper does nothing: the Recorder reports no file and line positions.
func (*Recorder) Helper() {}

// Fail records a failure.
func (r *Recorder) Fail() {
	r.failed = true
}

// FailNow records a failure and stops the goroutine started by Run.
func (r *Recorder) FailNow() {
	r.failed = true
	runtime.Goexit()
}

// Error records a failure with a message formatted like fmt.Sprint.
func (r *Recorder) Error(args ...any) {
	r.record(fmt.Sprint(args...))
}

// Errorf records a failure with a message formatted like fmt.Sprintf.
func (r *Recorder) Errorf(format string, args ...any) {
	r.record(fmt.Sprintf(format, args...))
}

// Fatal records a failure with a message formatted like fmt.Sprint and calls FailNow.
func (r *Recorder) Fatal(args ...any) {
	r.Error(args...)
	r.FailNow()
}

// Fatalf records a failure with a message formatted like fmt.Sprintf and calls FailNow.
func (r *Recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	r.FailNow()
}

func (r *Recorder) record(msg string) {
	r.failed = true
	r.messages = append(r.messages, msg)
}
