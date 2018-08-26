package store

import "errors"

// ErrInvalidNonce is returned when a signed request contains an invalid nonce.d
var ErrInvalidNonce = errors.New("invalid nonce")

// ErrUnregisteredNode is returned when an update is received for an unregistered node.
var ErrUnregisteredNode = errors.New("unregistered node")

// ErrMalformedNode is returned when the Node struct is incomplete or field values are invalid.
var ErrMalformedNode = errors.New("malformed node")
