package cookie

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the decoded identity from a hydration cookie.
//
// In JWT mode (COOKIE_ENCODING=jwt):
//   - HydrationToken is the opaque hyd_token; AppID identifies the issuing app.
//   - The hydrate handler resolves HydrationToken → {contextKey, claims} via Redis.
//
// In base64json mode (COOKIE_ENCODING=base64json):
//   - UserID is used directly as the contextKey (local dev / backward compat).
type Claims struct {
	// JWT mode
	HydrationToken string `json:"hyd_token,omitempty"`
	AppID          string `json:"app_id,omitempty"`

	// base64json mode
	UserID       string `json:"user_id,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
}

type Decoder struct {
	encoding string // "base64json" or "jwt"
	secret   []byte
}

func NewDecoder(encoding, secret string) *Decoder {
	return &Decoder{encoding: encoding, secret: []byte(secret)}
}

func (d *Decoder) Decode(raw string) (*Claims, error) {
	switch d.encoding {
	case "jwt":
		return d.decodeJWT(raw)
	default:
		return d.decodeBase64JSON(raw)
	}
}

func (d *Decoder) decodeBase64JSON(raw string) (*Claims, error) {
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
	}
	var c Claims
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if c.UserID == "" {
		return nil, fmt.Errorf("missing user_id in cookie")
	}
	return &c, nil
}

func (d *Decoder) decodeJWT(raw string) (*Claims, error) {
	type jwtClaims struct {
		HydrationToken string `json:"hyd_token"`
		AppID          string `json:"app_id"`
		jwt.RegisteredClaims
	}

	token, err := jwt.ParseWithClaims(raw, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return d.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt parse: %w", err)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid jwt claims")
	}
	if claims.HydrationToken == "" {
		return nil, fmt.Errorf("missing hyd_token in jwt")
	}
	if claims.AppID == "" {
		return nil, fmt.Errorf("missing app_id in jwt")
	}

	return &Claims{
		HydrationToken: claims.HydrationToken,
		AppID:          claims.AppID,
	}, nil
}
