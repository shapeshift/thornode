package types

import "errors"

type DbPrefix string

var (
	ErrVaultNotFound = errors.New("vault not found")
	ErrEventNotFound = errors.New("event not found")
)
