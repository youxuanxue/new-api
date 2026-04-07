//go:build tt
// +build tt

package service

import ttservice "github.com/QuantumNous/new-api/tt/service"

type VerifyResult = ttservice.VerifyResult

func VerifyModelAuthenticity(modelName string) (*VerifyResult, error) {
	return ttservice.VerifyModelAuthenticity(modelName)
}
