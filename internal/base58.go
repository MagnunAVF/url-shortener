package internal

import (
	"math/big"
	"strings"
)

// use Base58 (like Bitcoin)
const (
	alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	base     = 58
)

var bigBase = big.NewInt(base)
var bigZero = big.NewInt(0)

func EncodeID(id uint64) string {
	if id == 0 {
		return string(alphabet[0])
	}

	num := new(big.Int).SetUint64(id)

	var result strings.Builder
	remainder := new(big.Int)
	mod := new(big.Int)
	for num.Cmp(bigZero) > 0 {
		num.DivMod(num, bigBase, mod)
		remainder.Set(mod)
		result.WriteByte(alphabet[remainder.Int64()])
	}

	runes := []rune(result.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}
