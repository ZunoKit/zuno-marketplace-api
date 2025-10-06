package utils

import (
	"math/big"
)

func GetUint64Value(ptr *uint64, defaultValue uint64) uint64 {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func GetStringValue(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func ToBigInt(val uint64) *big.Int {
	return big.NewInt(int64(val))
}
