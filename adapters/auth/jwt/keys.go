package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// MinRSAKeyBits is the smallest RSA modulus accepted for signing or
// verification (NIST SP 800-57: 2048 bits).
const MinRSAKeyBits = 2048

// SigningKey is an asymmetric private key and the key id (kid) published in
// the token header and in the JWKS. Supported keys: *rsa.PrivateKey (RS256),
// *ecdsa.PrivateKey on P-256 (ES256) and ed25519.PrivateKey (EdDSA).
type SigningKey struct {
	ID  string
	Key crypto.Signer
}

// Keys configures asymmetric signing (NewWithKeys).
type Keys struct {
	// Signing signs every new token.
	Signing SigningKey
	// Verify lists extra public keys accepted on validation, by kid —
	// typically the previous signing key while its tokens are still alive.
	Verify map[string]crypto.PublicKey
}

// verifyKey is one accepted public key.
type verifyKey struct {
	pub crypto.PublicKey
	alg string
}

// keyring is the provider's asymmetric state (nil in HS256 mode).
type keyring struct {
	signID  string
	signer  crypto.Signer
	method  jwt.SigningMethod
	verify  map[string]verifyKey
	methods []string // distinct algorithms of verify, for the parser
}

// NewWithKeys creates a provider that signs with an asymmetric key instead
// of the shared HS256 secret (cfg.JWTSecret is ignored). Services that only
// validate tokens need just the public key — published by JWKS — and can no
// longer mint tokens, which a shared secret allows.
//
// Rotation: RotateSigningKey switches to a new key and keeps the old public
// key accepted; RetireKey drops it once its tokens have expired.
func NewWithKeys(cfg domain.AuthConfig, keys Keys) (*Provider, error) {
	kr := &keyring{verify: make(map[string]verifyKey)}
	if err := kr.setSigning(keys.Signing); err != nil {
		return nil, err
	}
	for kid, pub := range keys.Verify {
		if err := kr.addVerify(kid, pub); err != nil {
			return nil, err
		}
	}
	p := newProvider(cfg)
	p.keys = kr
	return p, nil
}

func (kr *keyring) setSigning(k SigningKey) error {
	if k.ID == "" {
		return errors.New("jwt: signing key needs an ID (kid)")
	}
	if k.Key == nil {
		return errors.New("jwt: signing key is nil")
	}
	method, err := methodFor(k.Key.Public())
	if err != nil {
		return err
	}
	if err := kr.addVerify(k.ID, k.Key.Public()); err != nil {
		return err
	}
	kr.signID, kr.signer, kr.method = k.ID, k.Key, method
	return nil
}

func (kr *keyring) addVerify(kid string, pub crypto.PublicKey) error {
	if kid == "" {
		return errors.New("jwt: verification key needs an ID (kid)")
	}
	method, err := methodFor(pub)
	if err != nil {
		return err
	}
	kr.verify[kid] = verifyKey{pub: pub, alg: method.Alg()}
	kr.refreshMethods()
	return nil
}

func (kr *keyring) refreshMethods() {
	seen := map[string]bool{}
	kr.methods = kr.methods[:0:0]
	for _, k := range kr.verify {
		if !seen[k.alg] {
			seen[k.alg] = true
			kr.methods = append(kr.methods, k.alg)
		}
	}
	sort.Strings(kr.methods)
}

// methodFor picks the signing algorithm of a public key and rejects weak or
// unsupported keys.
func methodFor(pub crypto.PublicKey) (jwt.SigningMethod, error) {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		if k.N.BitLen() < MinRSAKeyBits {
			return nil, fmt.Errorf("jwt: RSA key must be at least %d bits", MinRSAKeyBits)
		}
		return jwt.SigningMethodRS256, nil
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() {
			return nil, errors.New("jwt: ECDSA key must be on P-256 (ES256)")
		}
		return jwt.SigningMethodES256, nil
	case ed25519.PublicKey:
		return jwt.SigningMethodEdDSA, nil
	default:
		return nil, fmt.Errorf("jwt: unsupported key type %T (use RSA, ECDSA P-256 or Ed25519)", pub)
	}
}

// RotateSigningKey makes k the key that signs new tokens. The previous
// public key stays accepted, so tokens already issued remain valid; call
// RetireKey with its id after the refresh-token TTL has passed.
// It fails on a provider created with New (HS256).
func (p *Provider) RotateSigningKey(k SigningKey) error {
	p.keysMu.Lock()
	defer p.keysMu.Unlock()
	if p.keys == nil {
		return errors.New("jwt: RotateSigningKey requires a provider created with NewWithKeys")
	}
	return p.keys.setSigning(k)
}

// AddVerificationKey accepts tokens signed by another key (e.g. a key
// another instance already rotated to).
func (p *Provider) AddVerificationKey(kid string, pub crypto.PublicKey) error {
	p.keysMu.Lock()
	defer p.keysMu.Unlock()
	if p.keys == nil {
		return errors.New("jwt: AddVerificationKey requires a provider created with NewWithKeys")
	}
	return p.keys.addVerify(kid, pub)
}

// RetireKey stops accepting tokens signed by kid. The current signing key
// cannot be retired — rotate first.
func (p *Provider) RetireKey(kid string) error {
	p.keysMu.Lock()
	defer p.keysMu.Unlock()
	if p.keys == nil {
		return errors.New("jwt: RetireKey requires a provider created with NewWithKeys")
	}
	if kid == p.keys.signID {
		return errors.New("jwt: cannot retire the current signing key; rotate first")
	}
	delete(p.keys.verify, kid)
	p.keys.refreshMethods()
	return nil
}

// Asymmetric reports whether tokens are signed with an asymmetric key.
func (p *Provider) Asymmetric() bool {
	p.keysMu.RLock()
	defer p.keysMu.RUnlock()
	return p.keys != nil
}

// sign signs claims with the current key (HS256 secret or asymmetric key).
func (p *Provider) sign(claims jwt.MapClaims) (string, error) {
	p.keysMu.RLock()
	kr := p.keys
	var (
		method jwt.SigningMethod = jwt.SigningMethodHS256
		key    any               = p.secret
		kid    string
	)
	if kr != nil {
		method, key, kid = kr.method, kr.signer, kr.signID
	}
	p.keysMu.RUnlock()

	token := jwt.NewWithClaims(method, claims)
	if kid != "" {
		token.Header["kid"] = kid
	}
	return token.SignedString(key)
}

// verification returns the accepted algorithms and the key lookup used by
// the parser. In asymmetric mode the key is chosen by the kid header and its
// algorithm must be the one that key was registered with, so a token cannot
// pick a different algorithm for a known key (algorithm confusion).
func (p *Provider) verification() ([]string, jwt.Keyfunc) {
	p.keysMu.RLock()
	kr := p.keys
	if kr == nil {
		p.keysMu.RUnlock()
		return []string{"HS256"}, func(*jwt.Token) (any, error) { return p.secret, nil }
	}
	methods := append([]string(nil), kr.methods...)
	p.keysMu.RUnlock()

	return methods, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		p.keysMu.RLock()
		k, ok := p.keys.verify[kid]
		p.keysMu.RUnlock()
		if !ok || t.Method.Alg() != k.alg {
			return nil, errors.New("jwt: unknown key")
		}
		return k.pub, nil
	}
}

// JWKS returns the public keys as a JSON Web Key Set (RFC 7517), for a
// /.well-known/jwks.json endpoint. It is empty for an HS256 provider: a
// shared secret is never published.
func (p *Provider) JWKS() ([]byte, error) {
	type jwk struct {
		Kty string `json:"kty"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		N   string `json:"n,omitempty"`
		E   string `json:"e,omitempty"`
		Crv string `json:"crv,omitempty"`
		X   string `json:"x,omitempty"`
		Y   string `json:"y,omitempty"`
	}
	b64 := base64.RawURLEncoding.EncodeToString
	set := struct {
		Keys []jwk `json:"keys"`
	}{Keys: []jwk{}}

	p.keysMu.RLock()
	if p.keys != nil {
		for kid, k := range p.keys.verify {
			j := jwk{Use: "sig", Alg: k.alg, Kid: kid}
			switch pub := k.pub.(type) {
			case *rsa.PublicKey:
				j.Kty = "RSA"
				j.N = b64(pub.N.Bytes())
				j.E = b64(big.NewInt(int64(pub.E)).Bytes())
			case *ecdsa.PublicKey:
				size := (pub.Curve.Params().BitSize + 7) / 8
				j.Kty, j.Crv = "EC", "P-256"
				j.X = b64(pub.X.FillBytes(make([]byte, size)))
				j.Y = b64(pub.Y.FillBytes(make([]byte, size)))
			case ed25519.PublicKey:
				j.Kty, j.Crv = "OKP", "Ed25519"
				j.X = b64(pub)
			}
			set.Keys = append(set.Keys, j)
		}
	}
	p.keysMu.RUnlock()

	sort.Slice(set.Keys, func(i, j int) bool { return set.Keys[i].Kid < set.Keys[j].Kid })
	return json.Marshal(set)
}
