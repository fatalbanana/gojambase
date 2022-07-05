package jambase

import (
	"errors"
)

var (
	ErrNoExtension    = errors.New("header file doesn't have suitable extension")
	ErrNoJAMSignature = errors.New("missing JAM signature")
	ErrShortRead      = errors.New("read less data than expected")
)
