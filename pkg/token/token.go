package token

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
)

func Random(size int) (string, error) {
	token := make([]byte, size, size)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

func Name(clusterName string) string {
	return fmt.Sprintf("%s-token", clusterName)
}
