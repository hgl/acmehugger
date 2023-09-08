package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"io/fs"
	"log/slog"
	"os"
)

type KeyType int

const (
	KeyEC256 KeyType = iota
	KeyEC384
	KeyRSA2048
	KeyRSA3072
	KeyRSA4096
	KeyRSA8192
)

func ParseKeyType(s string) (KeyType, error) {
	switch s {
	case "ec256":
		return KeyEC256, nil
	case "ec384":
		return KeyEC384, nil
	case "rsa2048":
		return KeyRSA2048, nil
	case "rsa3072":
		return KeyRSA3072, nil
	case "rsa4096":
		return KeyRSA4096, nil
	case "rsa8192":
		return KeyRSA8192, nil
	default:
		return 0, errors.New("invalid key type: " + s)
	}
}

func (t KeyType) Size() int {
	switch t {
	case KeyEC256:
		return 256
	case KeyEC384:
		return 384
	case KeyRSA2048:
		return 2048
	case KeyRSA3072:
		return 3072
	case KeyRSA4096:
		return 4096
	case KeyRSA8192:
		return 8192
	default:
		panic("unknown KeyType")
	}
}

func NewKey(t KeyType) (crypto.PrivateKey, error) {
	switch t {
	case KeyEC256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case KeyEC384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case KeyRSA2048:
		return rsa.GenerateKey(rand.Reader, 2048)
	case KeyRSA3072:
		return rsa.GenerateKey(rand.Reader, 3072)
	case KeyRSA4096:
		return rsa.GenerateKey(rand.Reader, 4096)
	case KeyRSA8192:
		return rsa.GenerateKey(rand.Reader, 8192)
	default:
		panic("unknown KeyType")
	}
}

func LoadOrCreateKey(t KeyType, name string) (key crypto.PrivateKey, created bool, err error) {
	data, err := os.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
		data = nil
	} else if err != nil {
		return nil, false, err
	}
	if len(data) != 0 {
		switch t {
		case KeyEC256, KeyEC384:
			k, err := x509.ParseECPrivateKey(data)
			if err != nil {
				break
			}
			if k.Params().BitSize == t.Size() {
				slog.Debug("existing acme key found", "name", name, "keyType", t)
				return k, false, nil
			}
		case KeyRSA2048, KeyRSA3072, KeyRSA4096, KeyRSA8192:
			var k *rsa.PrivateKey
			k, err = x509.ParsePKCS1PrivateKey(data)
			if err != nil {
				break
			}
			if k.Size() == t.Size() {
				slog.Debug("existing acme key found", "name", name, "keyType", t)
				return k, false, nil
			}
		default:
			panic("unknown KeyType")
		}
		slog.Debug("acme key type changed, creating a new one")
	}

	key, err = NewKey(t)
	if err != nil {
		return nil, false, err
	}
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		data, err = x509.MarshalECPrivateKey(k)
	case *rsa.PrivateKey:
		data = x509.MarshalPKCS1PrivateKey(k)
	}
	if err != nil {
		return nil, false, err
	}
	err = os.WriteFile(name, data, 0600)
	if err != nil {
		return nil, false, err
	}
	slog.Debug("new acme key created", "name", name, "keyType", t)
	return key, true, nil
}
