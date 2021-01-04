package utils

import (
	uuid "github.com/satori/go.uuid"
)

func NewId() string {
	id := uuid.NewV1()
	//b64 := base64.URLEncoding.EncodeToString(id.Bytes()[:12])
	//return b64
	return id.String()
}
