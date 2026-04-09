//go:build !tt
// +build !tt

package model

// RunPostUserCreationHooks is a no-op in upstream builds.
func RunPostUserCreationHooks(userId int) {}
