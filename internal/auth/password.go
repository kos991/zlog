package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type SessionManager struct {
	secret []byte
}

func NewSessionManager(secret string) *SessionManager {
	return &SessionManager{secret: []byte(secret)}
}

func (m *SessionManager) Sign(username string) (string, error) {
	if m == nil || len(m.secret) == 0 {
		return "", errors.New("session secret not configured")
	}
	payload := username + "|" + simpleHMAC(m.secret, username)
	return payload, nil
}

func (m *SessionManager) Verify(token string) (string, error) {
	if m == nil || len(m.secret) == 0 {
		return "", errors.New("session secret not configured")
	}
	if token == "" {
		return "", errors.New("empty token")
	}
	idx := -1
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '|' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", errors.New("invalid token format")
	}
	username := token[:idx]
	sig := token[idx+1:]
	expected := simpleHMAC(m.secret, username)
	if sig != expected {
		return "", errors.New("invalid signature")
	}
	return username, nil
}

func simpleHMAC(key []byte, data string) string {
	const blockSize = 32
	opad := make([]byte, blockSize)
	ipad := make([]byte, blockSize)
	for i := range opad {
		opad[i] = 0x5c
		ipad[i] = 0x36
	}
	for i := 0; i < len(key) && i < blockSize; i++ {
		opad[i] ^= key[i]
		ipad[i] ^= key[i]
	}
	inner := sha256sum(append(ipad, []byte(data)...))
	outer := sha256sum(append(opad, []byte(inner)...))
	return outer
}

func sha256sum(data []byte) string {
	return hexEncode(sha256digest(data))
}
