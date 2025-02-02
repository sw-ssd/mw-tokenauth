package tokenauth_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/httptest"
	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"

	"github.com/gobuffalo/buffalo"
	tokenauth "github.com/gobuffalo/mw-tokenauth"
	"github.com/stretchr/testify/require"
)

func appHMAC() *buffalo.App {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}
	envy.Set("JWT_SECRET", "secret")
	a := buffalo.New(buffalo.Options{})
	// if method not specified it will use HMAC
	a.Use(tokenauth.New(tokenauth.Options{
		GetKey: tokenauth.GetHMACKey,
	}))
	a.GET("/", h)
	return a
}

func appRSA() *buffalo.App {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}
	envy.Set("JWT_PUBLIC_KEY", "test_certs/sample_key.pub")
	a := buffalo.New(buffalo.Options{})
	a.Use(tokenauth.New(tokenauth.Options{
		SignMethod: jwt.SigningMethodRS256,
	}))
	a.GET("/", h)
	return a
}

func appRSAPSS() *buffalo.App {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}
	envy.Set("JWT_PUBLIC_KEY", "test_certs/sample_key.pub")
	a := buffalo.New(buffalo.Options{})
	a.Use(tokenauth.New(tokenauth.Options{
		SignMethod: jwt.SigningMethodPS256,
		GetKey:     tokenauth.GetKeyRSAPSS,
	}))
	a.GET("/", h)
	return a
}

func appEdDSA() *buffalo.App {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}
	envy.Set("JWT_PUBLIC_KEY", "test_certs/ed25519-public.pem")

	a := buffalo.New(buffalo.Options{})
	a.Use(tokenauth.New(tokenauth.Options{
		SignMethod: jwt.SigningMethodEdDSA,
	}))
	a.GET("/", h)
	return a
}
func appECDSA() *buffalo.App {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}
	envy.Set("JWT_PUBLIC_KEY", "test_certs/ec256-public.pem")

	a := buffalo.New(buffalo.Options{})
	a.Use(tokenauth.New(tokenauth.Options{
		SignMethod: jwt.SigningMethodES256,
	}))
	a.GET("/", h)
	return a
}

func appCustomAuthScheme() *buffalo.App {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}
	envy.Set("JWT_SECRET", "secret")
	a := buffalo.New(buffalo.Options{})
	a.Use(tokenauth.New(tokenauth.Options{
		AuthScheme: "Token",
	}))
	a.GET("/", h)
	return a
}

// Test HMAC
func TestTokenHMAC(t *testing.T) {
	r := require.New(t)
	w := httptest.New(appHMAC())

	// Missing Authorization
	res := w.HTML("/").Get()
	r.Equal(http.StatusUnauthorized, res.Code)

	// invalid token
	req := w.HTML("/")
	req.Headers["Authorization"] = "badcreds"
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "token invalid")

	// expired token
	secretKey := envy.Get("JWT_SECRET", "secret")
	claims := jwt.MapClaims{}
	claims["sub"] = "1234567890"
	claims["exp"] = time.Now().Add(-time.Minute * 5).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secretKey))
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "Token is expired")

	// valid token
	claims["exp"] = time.Now().Add(time.Minute * 5).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ = token.SignedString([]byte(secretKey))
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusOK, res.Code)
}

// Test RSA
func TestTokenRSA(t *testing.T) {
	r := require.New(t)
	w := httptest.New(appRSA())

	// Missing Authorization
	res := w.HTML("/").Get()
	r.Equal(http.StatusUnauthorized, res.Code)

	// invalid token
	req := w.HTML("/")
	req.Headers["Authorization"] = "badcreds"
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "token invalid")

	// expired token
	privateKeyFile := envy.Get("JWT_PRIVATE_KEY", "test_certs/sample_key")
	key, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM(key)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error parsing key"))
	}
	claims := jwt.MapClaims{}
	claims["sub"] = "1234567890"
	claims["exp"] = time.Now().Add(-time.Minute * 5).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(parsedKey)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error signing token"))
	}
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "Token is expired")

	// valid token
	claims["exp"] = time.Now().Add(time.Minute * 5).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ = token.SignedString(parsedKey)
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusOK, res.Code)
}

// Test EdDSA
func TestTokenEdDSA(t *testing.T) {
	r := require.New(t)
	w := httptest.New(appEdDSA())

	// Missing Authorization
	res := w.HTML("/").Get()
	r.Equal(http.StatusUnauthorized, res.Code)

	// invalid token
	req := w.HTML("/")
	req.Headers["Authorization"] = "badcreds"
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "token invalid")

	// expired token
	privateKeyFile := envy.Get("JWT_PRIVATE_KEY", "test_certs/ed25519-private.pem")
	key, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error reading keyfile"))
	}
	parsedKey, err := jwt.ParseEdPrivateKeyFromPEM(key)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error parsing key"))
	}
	claims := jwt.MapClaims{}
	claims["sub"] = "1234567890"
	claims["exp"] = time.Now().Add(-time.Minute * 5).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, err := token.SignedString(parsedKey)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error signing token"))
	}
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "Token is expired")

	// valid token
	claims["exp"] = time.Now().Add(time.Minute * 5).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, _ = token.SignedString(parsedKey)
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusOK, res.Code)
}

// Test ECDSA
func TestTokenECDSA(t *testing.T) {
	r := require.New(t)
	w := httptest.New(appECDSA())

	// Missing Authorization
	res := w.HTML("/").Get()
	r.Equal(http.StatusUnauthorized, res.Code)

	// invalid token
	req := w.HTML("/")
	req.Headers["Authorization"] = "badcreds"
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "token invalid")

	// expired token
	privateKeyFile := envy.Get("JWT_PRIVATE_KEY", "test_certs/ec256-private.pem")
	key, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error reading keyfile"))
	}
	parsedKey, err := jwt.ParseECPrivateKeyFromPEM(key)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error parsing key"))
	}
	claims := jwt.MapClaims{}
	claims["sub"] = "1234567890"
	claims["exp"] = time.Now().Add(-time.Minute * 5).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(parsedKey)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error signing token"))
	}
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "Token is expired")

	// valid token
	claims["exp"] = time.Now().Add(time.Minute * 5).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, _ = token.SignedString(parsedKey)
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusOK, res.Code)
}

// Test RSAPSS
func TestTokenRSAPSS(t *testing.T) {
	r := require.New(t)
	w := httptest.New(appRSAPSS())

	// Missing Authorization
	res := w.HTML("/").Get()
	r.Equal(http.StatusUnauthorized, res.Code)

	// invalid token
	req := w.HTML("/")
	req.Headers["Authorization"] = "badcreds"
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "token invalid")

	// expired token
	privateKeyFile := envy.Get("JWT_PRIVATE_KEY", "test_certs/sample_key")
	key, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM(key)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error parsing key"))
	}
	claims := jwt.MapClaims{}
	claims["sub"] = "1234567890"
	claims["exp"] = time.Now().Add(-time.Minute * 5).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodPS256, claims)
	tokenString, err := token.SignedString(parsedKey)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error signing token"))
	}
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusUnauthorized, res.Code)
	r.Contains(res.Body.String(), "Token is expired")

	// valid token
	claims["exp"] = time.Now().Add(time.Minute * 5).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodPS256, claims)
	tokenString, _ = token.SignedString(parsedKey)
	req.Headers["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)
	res = req.Get()
	r.Equal(http.StatusOK, res.Code)
}

func TestAuthScheme(t *testing.T) {
	r := require.New(t)
	w := httptest.New(appCustomAuthScheme())

	req := w.HTML("/")
	claims := jwt.MapClaims{}
	claims["sub"] = "1234567890"
	claims["exp"] = time.Now().Add(time.Minute * 5).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secretKey := envy.Get("JWT_SECRET", "secret")
	tokenString, _ := token.SignedString([]byte(secretKey))
	req.Headers["Authorization"] = fmt.Sprintf("Token %s", tokenString)
	res := req.Get()
	r.Equal(http.StatusOK, res.Code)
}
