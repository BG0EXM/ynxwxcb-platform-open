package auth

import (
	"testing"
)

func TestGenerateAndParseToken(t *testing.T) {
	Init("super_secret_key")
	
	userID := int64(123)
	username := "testuser"
	realName := "Test User"
	roleCode := "admin"
	tokenVersion := 1
	
	token, err := GenerateToken(userID, username, realName, roleCode, tokenVersion)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if token == "" {
		t.Fatalf("GenerateToken returned empty string")
	}
	
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken error: %v", err)
	}
	
	if claims.UserID != userID {
		t.Errorf("expected UserID %v, got %v", userID, claims.UserID)
	}
	if claims.Username != username {
		t.Errorf("expected Username %v, got %v", username, claims.Username)
	}
	if claims.RealName != realName {
		t.Errorf("expected RealName %v, got %v", realName, claims.RealName)
	}
	if claims.RoleCode != roleCode {
		t.Errorf("expected RoleCode %v, got %v", roleCode, claims.RoleCode)
	}
	if claims.TokenVersion != tokenVersion {
		t.Errorf("expected TokenVersion %v, got %v", tokenVersion, claims.TokenVersion)
	}
}

func TestParseToken_InvalidToken(t *testing.T) {
	Init("super_secret_key")
	
	_, err := ParseToken("invalid.token.string")
	if err == nil {
		t.Errorf("expected error for invalid token string, got nil")
	}
}

func TestParseToken_InvalidSignature(t *testing.T) {
	Init("super_secret_key")
	token, _ := GenerateToken(1, "u", "r", "c", 1)
	
	// Change secret
	Init("another_secret_key")
	_, err := ParseToken(token)
	if err == nil {
		t.Errorf("expected error for token signed with different secret, got nil")
	}
}
