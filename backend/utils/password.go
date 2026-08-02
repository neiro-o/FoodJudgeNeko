package utils

import (
	"crypto/rand"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// randomPasswordAlphabet avoids visually ambiguous characters (0/O, 1/l/I)
// to keep generated passwords easy to read and retype.
const randomPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%^&*"

// GenerateRandomPassword returns a cryptographically random password of the
// given length, suitable for use as a one-time initial password.
func GenerateRandomPassword(length int) (string, error) {
	alphabetLen := big.NewInt(int64(len(randomPasswordAlphabet)))
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		result[i] = randomPasswordAlphabet[n.Int64()]
	}
	return string(result), nil
}

