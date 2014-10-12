package aws

import (
	"fmt"
	"os"
)

type Auth struct {
	AccessKey, SecretKey string
}

func EnvAuth() (Auth, error) {
	accessKey := os.Getenv("AWS_ACCESS_KEY")
	if accessKey == "" {
		return Auth{}, fmt.Errorf("AWS_ACCESS_KEY not defined")
	}
	secretKey := os.Getenv("AWS_SECRET_KEY")
	if secretKey == "" {
		return Auth{}, fmt.Errorf("AWS_SECRET_KEY not defined")
	}
	return Auth{accessKey, secretKey}, nil
}
