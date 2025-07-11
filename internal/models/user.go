package models

import (
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID              int64        `json:"id"`
	Username        string       `json:"username"`
	Email           string       `json:"email"`
	NormalizedEmail string       `json:"normalized_email"`
	Password        PasswordHash `json:"-"`
	CreatedAt       string       `json:"created_at"`
	UpdatedAt       string       `json:"updated_at"`
	IsActive        bool         `json:"is_active"`
	RoleID          int64        `json:"role_id"`
	Role            Role         `json:"role"`
}

type PasswordHash struct {
	Hash []byte
}

func (p *PasswordHash) Set(text string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(text), bcrypt.DefaultCost)

	if err != nil {
		return err
	}

	p.Hash = hash

	return nil
}

func (p *PasswordHash) Compare(password string) error {
	return bcrypt.CompareHashAndPassword(p.Hash, []byte(password))
}

// This method converts the PasswordHash into a []byte that the database can store.
// if the hash in the PasswordHash struct is private, make sure to uncomment this function
// func (p PasswordHash) Value() (driver.Value, error) {
// 	if p.Hash == nil {
// 		return nil, nil
// 	}
// 	return p.Hash, nil
// }
