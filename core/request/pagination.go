package request

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
)

const (
	// DefaultLimit is the page size when the request doesn't set one.
	DefaultLimit = 50
	// MaxLimit is the largest page size a request may set.
	MaxLimit = 100
)

// Pagination is the page of a collection requested by the offset and limit query parameters.
type Pagination struct {
	Offset int
	Limit  int
}

var errNotInteger = validation.NewError("validation_not_integer", "must be an integer")

// ParsePagination reads the offset and limit query parameters, defaulting to the first page of
// DefaultLimit resources. On invalid values it writes a 400 problem and returns false.
func ParsePagination(w http.ResponseWriter, r *http.Request) (Pagination, bool) {
	query := r.URL.Query()
	p := Pagination{Offset: 0, Limit: DefaultLimit}

	errs := validation.Errors{
		"offset": intParam(query, "offset", &p.Offset, validation.Min(0)),
		"limit":  intParam(query, "limit", &p.Limit, validation.Min(1), validation.Max(MaxLimit)),
	}
	if err := errs.Filter(); err != nil {
		writeQueryProblem(w, err)
		return Pagination{}, false
	}

	return p, true
}

// intParam parses the query parameter name into dst and validates it, leaving dst unchanged
// when the parameter is absent.
func intParam(query url.Values, name string, dst *int, rules ...validation.Rule) error {
	if !query.Has(name) {
		return nil
	}

	v, err := strconv.Atoi(query.Get(name))
	if err != nil {
		return errNotInteger
	}
	*dst = v

	return validation.Validate(v, rules...)
}

// invalidQueryProblem lists the invalid query parameters of a request in the
// envelope.ValidationErrorsExtension member, each located by its parameter name.
var invalidQueryProblem = envelope.DefineProblemType("invalid-query", "Invalid query parameters", http.StatusBadRequest)

func writeQueryProblem(w http.ResponseWriter, err error) {
	errs, ok := validationErrors(err, func(path []string) envelope.ValidationError {
		return envelope.ValidationError{Parameter: path[0]}
	})
	if !ok {
		panic(fmt.Errorf("request: validate query: %w", err))
	}

	envelope.WriteProblem(w, invalidQueryProblem.New(envelope.Occurrence{
		Detail:     "The request contains invalid query parameters.",
		Extensions: map[string]any{envelope.ValidationErrorsExtension: errs},
	}))
}
