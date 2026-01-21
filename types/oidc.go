package types

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
	ExtraParams  map[string]string
	Expiry       time.Duration
}

// OIDCClaims represents claims from an OIDC ID token.
type OIDCClaims struct {
	// Sub is the user's unique identifier at the provider.
	Sub string `json:"sub"`
	Iss string `json:"iss"`

	// Name is the user's full name.
	Name              string          `json:"name,omitempty"`
	Groups            []string        `json:"groups,omitempty"`
	Email             string          `json:"email,omitempty"`
	EmailVerified     FlexibleBoolean `json:"email_verified,omitempty"`
	ProfilePictureURL string          `json:"picture,omitempty"`
	Username          string          `json:"preferred_username,omitempty"`
}

// OIDCUserInfo represents additional user info from the userinfo endpoint.
type OIDCUserInfo struct {
	Sub               string          `json:"sub"`
	Name              string          `json:"name"`
	GivenName         string          `json:"given_name"`
	FamilyName        string          `json:"family_name"`
	PreferredUsername string          `json:"preferred_username"`
	Email             string          `json:"email"`
	EmailVerified     FlexibleBoolean `json:"email_verified,omitempty"`
	Picture           string          `json:"picture"`
}

// FlexibleBoolean handles JSON where boolean values may be strings.
// Some providers (like JumpCloud) return "true"/"false" as strings.
type FlexibleBoolean bool

func (bit *FlexibleBoolean) UnmarshalJSON(data []byte) error {
	var val any
	err := json.Unmarshal(data, &val)
	if err != nil {
		return fmt.Errorf("could not unmarshal data: %w", err)
	}

	switch v := val.(type) {
	case bool:
		*bit = FlexibleBoolean(v)
	case string:
		pv, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("could not parse %s as boolean: %w", v, err)
		}
		*bit = FlexibleBoolean(pv)
	default:
		return fmt.Errorf("could not parse %v as boolean", v)
	}

	return nil
}

// Identifier returns a unique identifier string combining the Iss and Sub claims.
func (c *OIDCClaims) Identifier() string {
	if c.Iss == "" && c.Sub == "" {
		return ""
	}
	if c.Iss == "" {
		return CleanIdentifier(c.Sub)
	}
	if c.Sub == "" {
		return CleanIdentifier(c.Iss)
	}

	issuer := c.Iss
	subject := c.Sub

	var result string
	// Try to parse as URL to handle URL joining correctly
	if u, err := url.Parse(issuer); err == nil && u.Scheme != "" {
		if joined, err := url.JoinPath(issuer, subject); err == nil {
			result = joined
		}
	}

	// If URL joining failed or issuer wasn't a URL, do simple string join
	if result == "" {
		issuer = strings.TrimSuffix(issuer, "/")
		subject = strings.TrimPrefix(subject, "/")
		result = issuer + "/" + subject
	}

	return CleanIdentifier(result)
}

// CleanIdentifier cleans a potentially malformed identifier by removing double slashes
// while preserving protocol specifications like http://.
func CleanIdentifier(identifier string) string {
	if identifier == "" {
		return identifier
	}

	identifier = strings.TrimSpace(identifier)

	// Handle URLs with schemes
	u, err := url.Parse(identifier)
	if err == nil && u.Scheme != "" {
		parts := strings.FieldsFunc(u.Path, func(c rune) bool { return c == '/' })
		for i, part := range parts {
			parts[i] = strings.TrimSpace(part)
		}
		cleanParts := make([]string, 0, len(parts))
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}

		if len(cleanParts) == 0 {
			u.Path = ""
		} else {
			u.Path = "/" + strings.Join(cleanParts, "/")
		}
		u.Scheme = strings.ToLower(u.Scheme)
		return u.String()
	}

	// Handle non-URL identifiers
	parts := strings.FieldsFunc(identifier, func(c rune) bool { return c == '/' })
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			cleanParts = append(cleanParts, trimmed)
		}
	}
	if len(cleanParts) == 0 {
		return ""
	}
	return strings.Join(cleanParts, "/")
}
