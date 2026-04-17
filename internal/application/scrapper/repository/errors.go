package repository

import "errors"

var ErrEntityAlreadyExists = errors.New("entity already exists")
var ErrEntityNotFound = errors.New("entity not found")
var ErrUnknownDBError = errors.New("unknown database error")
var ErrBadParams = errors.New("bad parameters")
