package validator

// Options configures StructValidator behavior.
type Options struct {
	// StopOnFirstError stops at the first ValidationError (tags).
	// Custom model Validate() does not run if a tag error already exists.
	StopOnFirstError bool

	// StrictUnknown, if true, unregistered rules yield Rule: "unknown".
	// Default true (via DefaultOptions / NewValidator).
	// Mirrored on Registry.StrictUnknown during Validate.
	StrictUnknown bool
}

// DefaultOptions returns Options with StrictUnknown=true.
func DefaultOptions() Options {
	return Options{StrictUnknown: true}
}
