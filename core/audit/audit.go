// Package audit holds what the audit fields of all features share.
package audit

import "uuid"

// SystemActor returns the actor that attributes changes to pkgsilo itself rather than to an
// identity: the Nil UUID, which no identity has.
func SystemActor() uuid.UUID {
	return uuid.Nil()
}
