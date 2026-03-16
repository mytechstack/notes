package cookie

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestDecodeBase64JSON(t *testing.T) {
	claims := Claims{UserID: "u123", SessionToken: "tok456"}
	b, _ := json.Marshal(claims)
	encoded := base64.StdEncoding.EncodeToString(b)

	d := NewDecoder("base64json", "")
	got, err := d.Decode(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "u123" {
		t.Errorf("user_id: got %q, want %q", got.UserID, "u123")
	}
}

func TestDecodeBase64JSON_URLEncoding(t *testing.T) {
	claims := Claims{UserID: "u123"}
	b, _ := json.Marshal(claims)
	encoded := base64.URLEncoding.EncodeToString(b)

	d := NewDecoder("base64json", "")
	got, err := d.Decode(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "u123" {
		t.Errorf("user_id: got %q, want %q", got.UserID, "u123")
	}
}

func TestDecodeBase64JSON_MissingUserID(t *testing.T) {
	b, _ := json.Marshal(map[string]string{"session_token": "tok"})
	encoded := base64.StdEncoding.EncodeToString(b)

	d := NewDecoder("base64json", "")
	_, err := d.Decode(encoded)
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestDecodeBase64JSON_InvalidBase64(t *testing.T) {
	d := NewDecoder("base64json", "")
	_, err := d.Decode("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeJWT(t *testing.T) {
	secret := "test-secret"
	type jwtC struct {
		UserID       string `json:"user_id"`
		SessionToken string `json:"session_token"`
		jwt.RegisteredClaims
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtC{
		UserID:       "u999",
		SessionToken: "s1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	d := NewDecoder("jwt", secret)
	got, err := d.Decode(signed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "u999" {
		t.Errorf("user_id: got %q, want %q", got.UserID, "u999")
	}
}

func TestDecodeJWT_WrongSecret(t *testing.T) {
	type jwtC struct {
		UserID string `json:"user_id"`
		jwt.RegisteredClaims
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtC{UserID: "u1"})
	signed, _ := token.SignedString([]byte("real-secret"))

	d := NewDecoder("jwt", "wrong-secret")
	_, err := d.Decode(signed)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
