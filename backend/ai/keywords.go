package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

// negativeKeywordRulePlaceholder is the token embedded in
// prompts/user_profile_summary_v1.md that gets substituted with the decrypted
// rule text at prompt-build time (see BuildUserProfileSummaryPrompt).
const negativeKeywordRulePlaceholder = "{{NEGATIVE_KEYWORD_RULE}}"

// negativeKeywordRuleKeyB64 and negativeKeywordRuleBlobB64 hold an
// AES-256-GCM-encrypted copy of the "黑话规则" paragraph, which names the
// specific slur/dogwhistle tokens that force a negative classification.
//
// This is deliberately kept out of plaintext (both here and in the .md
// prompt file) so that the exact trigger words are not literally searchable
// via GitHub code search or a plain `grep` of this public repo — someone
// scanning for those tokens to evade detection, or to target this project,
// won't find them as readable strings in source control. The key lives
// alongside the ciphertext (as it must, since this is open source), so this
// is obfuscation against casual/automated text search, not a security
// boundary against a determined reader of the source.
//
// Regenerate both values together if the rule text changes; see the comment
// on decryptNegativeKeywordRule for the expected plaintext shape.
const (
	negativeKeywordRuleKeyB64  = "eCZgRc4KpM+kq5pX7hxa1s6D1DtXf+0aV7m4uttxQy0="
	negativeKeywordRuleBlobB64 = "o8AJq9buPczZfs18iccv6mYiIKBfgK5GPIx+jZBRI/Bf7RMdH3norL0TnbICnHHFgOIm2iTjJ31b7WnuNDYzcIbHDASb67Gq1RWbyjU7gCE7qvtQ5xxkR4Pxh3YRIfERtIniFPc/qGR234+Paqo6o/YvNqADmlQJwu7HTanBBo2B7DEQebbKO3gCEaXX310xJLicMrI9MKxVIJayEuXVSOdqrKzpW0qesy0s18loOX/F+H9/hSz71AHKr9rYv+w/Y+pUmjodkbpW8nH1XRnOOQb9ex5QmBdLZrqk26Kad6yjQ6hQS3t4301RdRDjxBU7oe6btTc6QEaHUr77K1Ww07yXoMhtq/ARDQi4N1AuMvfo04Y8tZ2itNe7N9wDQM4HtGj3CWHYH6/6eOG0iNqArr8gT1uMF6Ly44VRnzvlLpzKProxqgWY4XhG8w6zP9/V4nbeXXA6O3B6JKXvC0N8zLCxO8xsten6NWw8MUYLUr+ZLqeEZFLHmE2yXjKlf/zc+qGnQtgxcWeUoVQMNhBVAZKlVgX1dtsE24iYCotNFdWf2JczxBAbETWL5iam1XRscz9JlCV2FBC2L2Nap0gPd2Tg7IKqM7ZdpqePzpReT4UVPji/sh8oFps913mJfBD9qD/LWv74ldC3R3G6CH8="
)

// decryptNegativeKeywordRule decrypts the embedded negative-keyword rule
// paragraph (AES-256-GCM; nonce is the first gcm.NonceSize() bytes of the
// decoded blob). The plaintext is the same "黑话规则：..." paragraph that
// used to live directly in the .md prompt file.
func decryptNegativeKeywordRule() (string, error) {
	key, err := base64.StdEncoding.DecodeString(negativeKeywordRuleKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode negative keyword rule key: %w", err)
	}
	blob, err := base64.StdEncoding.DecodeString(negativeKeywordRuleBlobB64)
	if err != nil {
		return "", fmt.Errorf("decode negative keyword rule blob: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("init gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return "", fmt.Errorf("ciphertext blob shorter than nonce size")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt negative keyword rule: %w", err)
	}
	return string(plaintext), nil
}
