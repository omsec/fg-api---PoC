package helpers

import "golang.org/x/crypto/bcrypt"

// Passwörter sollten nicht verschlüsselt, sondern hashed werden
// https://astaxie.gitbooks.io/build-web-application-with-golang/content/en/09.5.html

// GenerateHash from a password
func GenerateHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil
	}
	return string(hash), nil
}

// CompareHash checks if a password matches a hash value
func CompareHash(hash string, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return false, err
	}
	return true, nil
}
