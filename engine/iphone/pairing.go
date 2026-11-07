package iphone

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// newPairing makes the certificates of a pairing: a root of this
// computer's own, which signs a certificate for the computer and one for
// the phone's public key.
func newPairing(l *lockdown) (map[string]any, error) {
	pubPEM, _ := l.value("", "DevicePublicKey").([]byte)
	block, _ := pem.Decode(pubPEM)
	if block == nil {
		return nil, fmt.Errorf("the iPhone didn't give its public key")
	}
	devKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		k, err2 := x509.ParsePKIXPublicKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("reading the iPhone's public key: %w", err)
		}
		var ok bool
		if devKey, ok = k.(*rsa.PublicKey); !ok {
			return nil, fmt.Errorf("the iPhone's public key isn't RSA")
		}
	}
	buid, err := systemBUID()
	if err != nil {
		return nil, err
	}
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	now := time.Now().Add(-time.Hour)
	tmpl := func(serial int64, ca bool) *x509.Certificate {
		c := &x509.Certificate{
			SerialNumber:          big.NewInt(serial),
			Subject:               pkix.Name{},
			NotBefore:             now,
			NotAfter:              now.AddDate(10, 0, 0),
			BasicConstraintsValid: true,
			IsCA:                  ca,
			SignatureAlgorithm:    x509.SHA256WithRSA,
		}
		if ca {
			c.KeyUsage = x509.KeyUsageCertSign
		} else {
			c.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
		}
		return c
	}
	root := tmpl(0, true)
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	rootCert, _ := x509.ParseCertificate(rootDER)
	hostDER, err := x509.CreateCertificate(rand.Reader, tmpl(0, false), rootCert, &hostKey.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	devDER, err := x509.CreateCertificate(rand.Reader, tmpl(0, false), rootCert, devKey, rootKey)
	if err != nil {
		return nil, err
	}
	certPEM := func(der []byte) []byte { return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}) }
	keyPEM := func(k *rsa.PrivateKey) []byte {
		return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	}
	return map[string]any{
		"DeviceCertificate": certPEM(devDER),
		"HostCertificate":   certPEM(hostDER),
		"RootCertificate":   certPEM(rootDER),
		"HostPrivateKey":    keyPEM(hostKey),
		"RootPrivateKey":    keyPEM(rootKey),
		"HostID":            newUUID(),
		"SystemBUID":        buid,
	}, nil
}

// newUUID is a random UUID, upper case, as lockdown writes them.
func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]))
}
