package transform

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

const (
	digitsAlphabet   = "0123456789"
	alphaNumAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// RandomDigits returns n cryptographically random ASCII digits.
// n == 0 yields ""; n < 0 is an error.
func RandomDigits(n int) (string, error) {
	return randomFromAlphabet(n, digitsAlphabet)
}

// RandomAlphaNum returns n cryptographically random alphanumeric characters (0-9A-Za-z).
func RandomAlphaNum(n int) (string, error) {
	return randomFromAlphabet(n, alphaNumAlphabet)
}

// RandomHex returns a hex string of nBytes random bytes (length 2*nBytes).
func RandomHex(nBytes int) (string, error) {
	if nBytes < 0 {
		return "", fmt.Errorf("transform: RandomHex: nBytes must be >= 0")
	}
	if nBytes == 0 {
		return "", nil
	}
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomFromAlphabet(n int, alphabet string) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("transform: random: n must be >= 0")
	}
	if n == 0 {
		return "", nil
	}
	mod := big.NewInt(int64(len(alphabet)))
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, mod)
		if err != nil {
			return "", err
		}
		out[i] = alphabet[v.Int64()]
	}
	return string(out), nil
}
