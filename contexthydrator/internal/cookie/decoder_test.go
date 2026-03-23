package cookie

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestDecodeBase64JSON(t *testing.T) {
	claims := map[string]string{"user_id": "u123", "session_token": "tok456"}
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
	claims := map[string]string{"user_id": "u123"}
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
		HydrationToken string `json:"hyd_token"`
		AppID          string `json:"app_id"`
		jwt.RegisteredClaims
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtC{
		HydrationToken: "opaque-token-xyz",
		AppID:          "test-app",
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
	if got.HydrationToken != "opaque-token-xyz" {
		t.Errorf("hyd_token: got %q, want %q", got.HydrationToken, "opaque-token-xyz")
	}
	if got.AppID != "test-app" {
		t.Errorf("app_id: got %q, want %q", got.AppID, "test-app")
	}
}

func TestDecodeJWT_WrongSecret(t *testing.T) {
	type jwtC struct {
		HydrationToken string `json:"hyd_token"`
		AppID          string `json:"app_id"`
		jwt.RegisteredClaims
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtC{
		HydrationToken: "tok",
		AppID:          "app",
	})
	signed, _ := token.SignedString([]byte("real-secret"))

	d := NewDecoder("jwt", "wrong-secret")
	_, err := d.Decode(signed)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
