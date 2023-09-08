package util

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
)

func Pointer[T any](s T) *T {
	return &s
}

func FileExist(name string) (bool, error) {
	_, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// TODO: no-op if content is correct
func ForceSymlink(oldname, newname string) error {
	_, err := os.Lstat(newname)
	if err == nil {
		err = os.Remove(newname)
		if err != nil {
			return err
		}
	}

	return os.Symlink(oldname, newname)
}

func ReadText(name string) (string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(data), err
}

func ReadJSON(name string, v any) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	return dec.Decode(&v)
}

func WriteJSON(name string, v any, perm os.FileMode) (err error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer func() {
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
	}()
	enc := json.NewEncoder(f)
	err = enc.Encode(v)
	return
}

func ReadCert(name string) (cert *x509.Certificate, err error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return
	}
	block, _ := pem.Decode(data)
	cert, err = x509.ParseCertificate(block.Bytes)
	return
}
