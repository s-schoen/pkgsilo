package envelope

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ProblemJSONMediaType is the media type of an RFC 9457 Problem Details JSON response.
const ProblemJSONMediaType = "application/problem+json"

// AboutBlankProblemType is the problem type of errors with no semantics beyond their HTTP status.
const AboutBlankProblemType = "about:blank"

// ProblemType holds the metadata shared by every occurrence of a problem.
type ProblemType struct {
	// Type is a stable URI reference that identifies and preferably documents the problem type.
	Type string
	// Title is a short summary that stays the same across occurrences.
	Title string
	// Status is the HTTP status of every occurrence.
	Status int
}

// DefineProblemType returns a problem type, meant to be declared once as a package-level variable.
//
// It panics if status is not between 100 and 599 or title is empty.
func DefineProblemType(typ, title string, status int) ProblemType {
	if status < 100 || status > 599 {
		panic(fmt.Sprintf("envelope: problem type status %d is not between 100 and 599", status))
	}
	if title == "" {
		panic("envelope: problem type title must not be empty")
	}

	return ProblemType{Type: typ, Title: title, Status: status}
}

// Occurrence holds the values specific to one occurrence of a problem.
type Occurrence struct {
	// Detail explains this occurrence to a human.
	Detail string
	// Instance is a URI reference identifying this occurrence.
	Instance string
	// Extensions are additional top-level members defined by the problem type.
	Extensions map[string]any
}

// New returns an occurrence of t.
func (t ProblemType) New(o Occurrence) Problem {
	return Problem{
		Type:       t.Type,
		Title:      t.Title,
		Status:     t.Status,
		Detail:     o.Detail,
		Instance:   o.Instance,
		Extensions: o.Extensions,
	}
}

// Problem is an RFC 9457 Problem Details response body.
type Problem struct {
	Type     string
	Title    string
	Status   int
	Detail   string
	Instance string
	// Extensions are encoded as top-level members. Keys of standard members are ignored.
	Extensions map[string]any
}

type problemMembers struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// MarshalJSON encodes the standard members first, followed by the extensions sorted by key.
func (p Problem) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(problemMembers{
		Type:     p.Type,
		Title:    p.Title,
		Status:   p.Status,
		Detail:   p.Detail,
		Instance: p.Instance,
	})
	if err != nil {
		return nil, err
	}

	ext := make(map[string]any, len(p.Extensions))
	for k, v := range p.Extensions {
		switch k {
		case "type", "title", "status", "detail", "instance":
			continue
		}
		ext[k] = v
	}
	if len(ext) == 0 {
		return b, nil
	}

	e, err := json.Marshal(ext)
	if err != nil {
		return nil, err
	}

	// Merge both objects: drop the closing brace of the first and the opening brace of the second.
	return append(append(b[:len(b)-1], ','), e[1:]...), nil
}

// WriteProblem writes p as a Problem Details response whose HTTP status matches p.Status.
func WriteProblem(w http.ResponseWriter, p Problem) {
	writeJSON(w, p.Status, ProblemJSONMediaType, p)
}

func defineAboutBlankProblem(status int) ProblemType {
	return DefineProblemType(AboutBlankProblemType, http.StatusText(status), status)
}

// Generic HTTP errors, using the about:blank problem type.
var (
	BadRequestProblem            = defineAboutBlankProblem(http.StatusBadRequest)
	UnauthorizedProblem          = defineAboutBlankProblem(http.StatusUnauthorized)
	ForbiddenProblem             = defineAboutBlankProblem(http.StatusForbidden)
	NotFoundProblem              = defineAboutBlankProblem(http.StatusNotFound)
	MethodNotAllowedProblem      = defineAboutBlankProblem(http.StatusMethodNotAllowed)
	ConflictProblem              = defineAboutBlankProblem(http.StatusConflict)
	RequestEntityTooLargeProblem = defineAboutBlankProblem(http.StatusRequestEntityTooLarge)
	UnsupportedMediaTypeProblem  = defineAboutBlankProblem(http.StatusUnsupportedMediaType)
	UnprocessableEntityProblem   = defineAboutBlankProblem(http.StatusUnprocessableEntity)
	InternalServerErrorProblem   = defineAboutBlankProblem(http.StatusInternalServerError)
)

// ValidationErrorsExtension is the problem extension member that lists ValidationErrors.
const ValidationErrorsExtension = "errors"

// ValidationError describes one invalid value of a request. Exactly one of Pointer and
// Parameter locates the value.
type ValidationError struct {
	Detail string `json:"detail"`
	// Pointer is a JSON Pointer (RFC 6901) to the invalid member of the request body, in URI
	// fragment form such as "#/displayName".
	Pointer string `json:"pointer,omitempty"`
	// Parameter is the name of the invalid query parameter.
	Parameter string `json:"parameter,omitempty"`
}
