// Package tokenauth provides jwt token authorisation middleware
// supports HMAC, RSA, ECDSA, RSAPSS EdDSA algorithms
// uses github.com/golang-jwt/jwt/v4 for jwt implementation
//
// Setting Up tokenauth middleware
//
// Using tokenauth with defaults
//  app.Use(tokenauth.New(tokenauth.Options{}))
// Specifying Signing method for JWT
//  app.Use(tokenauth.New(tokenauth.Options{
//      SignMethod: jwt.SigningMethodRS256,
//  }))
// By default the Key used is loaded from the JWT_SECRET or JWT_PUBLIC_KEY env variable depending
// on the SigningMethod used. However you can retrive the key from a different source.
//  app.Use(tokenauth.New(tokenauth.Options{
//      GetKey: func(jwt.SigningMethod) (interface{}, error) {
//           // Your Implementation here ...
//      },
//  }))
// Default authorisation scheme is Bearer, you can specify your own.
//  app.Use(tokenauth.New(tokenauth.Options{
//      AuthScheme: "Token"
//  }))
//
//
// Creating a new token
//
// This can be referred from the underlying JWT package being used https://github.com/golang-jwt/jwt
//
// Example
//  claims := jwt.MapClaims{}
//  claims["userid"] = "123"
//  claims["exp"] = time.Now().Add(time.Minute * 5).Unix()
//  // add more claims
//  token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
//  tokenString, err := token.SignedString([]byte(SecretKey))
//
//
// Getting Claims from JWT token from buffalo context
//
// Example of retriving username from claims (this step is same regardless of the signing method used)
//  claims := c.Value("claims").(jwt.MapClaims)
//  username := claims["username"].(string)
package tokenauth

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/envy"
	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
)

var (
	// ErrTokenInvalid is returned when the token provided is invalid
	ErrTokenInvalid = errors.New("token invalid")
	// ErrNoToken is returned if no token is supplied in the request.
	ErrNoToken = errors.New("token not found in request")
	// ErrBadSigningMethod is returned if the token sign method in the request
	// does not match the signing method used
	ErrBadSigningMethod = errors.New("unexpected signing method")
)

// Options for the JWT middleware
type Options struct {
	SignMethod jwt.SigningMethod
	GetKey     func(jwt.SigningMethod) (interface{}, error)
	AuthScheme string
}

// New enables jwt token verification if no Sign method is provided,
// by default uses HMAC
func New(options Options) buffalo.MiddlewareFunc {
	// set sign method to HMAC if not provided
	if options.SignMethod == nil {
		options.SignMethod = jwt.SigningMethodHS256
	}
	if options.GetKey == nil {
		options.GetKey = selectGetKeyFunc(options.SignMethod)
	}
	// get key for validation
	key, err := options.GetKey(options.SignMethod)
	// if error on getting key exit.
	if err != nil {
		log.Fatal(errors.Wrap(err, "couldn't get key"))
	}
	if options.AuthScheme == "" {
		options.AuthScheme = "Bearer"
	}
	return func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			// get Authorisation header value
			authString := c.Request().Header.Get("Authorization")

			tokenString, err := getJwtToken(authString, options.AuthScheme)
			// if error on getting the token, return with status unauthorized
			if err != nil {
				return c.Error(http.StatusUnauthorized, err)
			}

			// validating and parsing the tokenString
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Validating if algorithm used for signing is same as the algorithm in token
				if token.Method.Alg() != options.SignMethod.Alg() {
					return nil, ErrBadSigningMethod
				}
				return key, nil
			})
			// if error validating jwt token, return with status unauthorized
			if err != nil {
				return c.Error(http.StatusUnauthorized, err)
			}

			// set the claims as context parameter.
			// so that the actions can use the claims from jwt token
			c.Set("claims", token.Claims)
			// calling next handler
			err = next(c)

			return err
		}
	}
}

// selectGetKeyFunc is an helper function to choose the GetKey function
// according to the Signing method used
func selectGetKeyFunc(method jwt.SigningMethod) func(jwt.SigningMethod) (interface{}, error) {
	switch method.(type) {
	case *jwt.SigningMethodRSA:
		return GetKeyRSA
	case *jwt.SigningMethodECDSA:
		return GetKeyECDSA
	case *jwt.SigningMethodRSAPSS:
		return GetKeyRSAPSS
	case *jwt.SigningMethodEd25519:
		return GetkeyEdDSA
	default:
		return GetHMACKey
	}
}

// GetHMACKey gets secret key from env
func GetHMACKey(jwt.SigningMethod) (interface{}, error) {
	key, err := envy.MustGet("JWT_SECRET")
	return []byte(key), err
}

// GetKeyRSA gets the public key file location from env and returns rsa.PublicKey
func GetKeyRSA(jwt.SigningMethod) (interface{}, error) {
	key, err := envy.MustGet("JWT_PUBLIC_KEY")
	if err != nil {
		return nil, err
	}
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPublicKeyFromPEM(keyData)
}

// GetKeyRSAPSS uses GetKeyRSA() since both requires rsa.PublicKey
func GetKeyRSAPSS(signingMethod jwt.SigningMethod) (interface{}, error) {
	return GetKeyRSA(signingMethod)
}

// GetKeyECDSA gets the public.pem file location from env and returns ecdsa.PublicKey
func GetKeyECDSA(jwt.SigningMethod) (interface{}, error) {
	key, err := envy.MustGet("JWT_PUBLIC_KEY")
	if err != nil {
		return nil, err
	}
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return nil, err
	}
	return jwt.ParseECPublicKeyFromPEM(keyData)
}

// GetKeyECDSA gets the public.pem file location from env and returns eddsa.PublicKey
func GetkeyEdDSA(jwt.SigningMethod) (interface{}, error) {
	key, err := envy.MustGet("JWT_PUBLIC_KEY")
	if err != nil {
		return nil, err
	}
	keyData, err := ioutil.ReadFile(key)
	if err != nil {
		return nil, err
	}
	return jwt.ParseEdPublicKeyFromPEM(keyData)
}

// getJwtToken gets the token from the Authorisation header
// removes the given authorisation scheme part (e.g. Bearer) from the authorisation header value.
// returns No token error if Token is not found
// returns Token Invalid error if the token value cannot be obtained by removing authorisation scheme part (e.g. `Bearer `)
func getJwtToken(authString, authScheme string) (string, error) {
	if authString == "" {
		return "", ErrNoToken
	}
	l := len(authScheme)
	if len(authString) > l+1 && authString[:l] == authScheme {
		return authString[l+1:], nil
	}
	return "", ErrTokenInvalid
}
