package testutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type Certificate struct {
	CertPath string
	KeyPath  string
}

func newCertificate(parentDir string, kind string, cert *x509.Certificate, key *ecdsa.PrivateKey) (Certificate, error) {
	certPath := filepath.Join(parentDir, kind+"-cert.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		return Certificate{}, err
	}
	defer certFile.Close()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err != nil {
		return Certificate{}, err
	}

	keyPath := filepath.Join(parentDir, kind+"-key.pem")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return Certificate{}, err
	}
	defer keyFile.Close()

	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return Certificate{}, err
	}
	pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	return Certificate{
		CertPath: certPath,
		KeyPath:  keyPath,
	}, nil
}

func GenCertificate(t *testing.T) (ca Certificate, server Certificate) {
	caCert, caKey, err := generateCA()
	if err != nil {
		t.Fatal(err)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Example Inc"}},
		DNSNames:              []string{"example.com", "www.example.com"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	serverCert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		t.Fatal(err)
	}

	ca, err = newCertificate(t.TempDir(), "ca", caCert, caKey)
	if err != nil {
		t.Fatal(err)
	}

	server, err = newCertificate(t.TempDir(), "server", serverCert, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	return ca, server
}

func generateCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, privateKey, nil
}
