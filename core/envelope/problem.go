package envelope

import (
	"encoding/json/jsontext"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
)

// ProblemJSONMediaType is the media type of an RFC 9457 Problem Details JSON response.
const ProblemJSONMediaType = "application/problem+json"

// AboutBlankProblemType is the problem type of errors with no semantics beyond their HTTP status.
const AboutBlankProblemType = "about:blank"

// problemTypeBaseURI prefixes the slug of every defined problem type. Clients compare type URIs
// as identifiers, so it must stay the same across deployments.
const problemTypeBaseURI = "https://pkgsilo/problems/"

var problemTypeSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ProblemType holds the metadata shared by every occurrence of a problem.
type ProblemType struct {
	// Type is a stable URI reference that identifies and preferably documents the problem type.
	Type string
	// Title is a short summary that stays the same across occurrences.
	Title string
	// Status is the HTTP status of every occurrence.
	Status int
}

// DefineProblemType returns the problem type identified by slug, meant to be declared once as a
// package-level variable. Define one for every problem a client must tell apart from others with
// the same status; the generic problems below cover the rest.
//
// Its URI is the slug under https://pkgsilo/problems/. It panics if slug isn't lowercase
// kebab-case, status is not between 100 and 599 or title is empty.
func DefineProblemType(slug, title string, status int) ProblemType {
	if !problemTypeSlugPattern.MatchString(slug) {
		panic(fmt.Sprintf("envelope: problem type slug %q is not lowercase kebab-case", slug))
	}
	if status < 100 || status > 599 {
		panic(fmt.Sprintf("envelope: problem type status %d is not between 100 and 599", status))
	}
	if title == "" {
		panic("envelope: problem type title must not be empty")
	}

	return ProblemType{Type: problemTypeBaseURI + slug, Title: title, Status: status}
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

// standardMembers are the members of every problem, which an extension can't replace.
var standardMembers = []string{"type", "title", "status", "detail", "instance"}

// MarshalJSONTo encodes the standard members first, followed by the extensions sorted by key. It leaves out
// an empty detail and instance.
func (p Problem) MarshalJSONTo(enc *jsontext.Encoder) error {
	members := []member{{name: "type", value: p.Type}, {name: "title", value: p.Title}, {name: "status", value: p.Status}}
	if p.Detail != "" {
		members = append(members, member{name: "detail", value: p.Detail})
	}
	if p.Instance != "" {
		members = append(members, member{name: "instance", value: p.Instance})
	}
	for _, name := range slices.Sorted(maps.Keys(p.Extensions)) {
		if !slices.Contains(standardMembers, name) {
			members = append(members, member{name: name, value: p.Extensions[name]})
		}
	}

	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return fmt.Errorf("encode problem: %w", err)
	}
	for _, m := range members {
		if err := encodeMember(enc, m); err != nil {
			return fmt.Errorf("encode problem: %w", err)
		}
	}
	if err := enc.WriteToken(jsontext.EndObject); err != nil {
		return fmt.Errorf("encode problem: %w", err)
	}

	return nil
}

// WriteProblem writes p as a Problem Details response whose HTTP status matches p.Status.
func WriteProblem(w http.ResponseWriter, p Problem) {
	writeJSON(w, p.Status, ProblemJSONMediaType, p)
}

func defineAboutBlankProblem(status int) ProblemType {
	return ProblemType{Type: AboutBlankProblemType, Title: http.StatusText(status), Status: status}
}

// Generic HTTP errors, using the about:blank problem type. They're meant for problems that mean
// nothing beyond their status; DefineProblemType defines the others.
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
