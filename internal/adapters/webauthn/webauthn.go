// Copyright 2026 IntegrAllTech Ltda.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package webauthn adapts the go-webauthn library (already in the tree) to the
// ArchGuard identity/factor model (pacote 005 T-002). WebAuthn is the default,
// phishing-resistant factor (ADR-0010). This adapter runs the registration and
// authentication ceremonies and maps their results onto domain.Credential —
// storing only PUBLIC material (INV-7) and assigning the assurance level: a
// user-verified, non-backup-eligible (hardware) authenticator is AAL3; a synced
// or non-user-verified passkey is AAL2.
package webauthn

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

// Service wraps a configured WebAuthn relying party.
type Service struct {
	wa *webauthn.WebAuthn
}

// NewService builds the relying party from its display name, RP ID (the domain,
// no scheme/port) and the permitted origins. WebAuthn requires a stable RP ID
// and HTTPS origins (ADR-0010 / RFC-0006).
func NewService(rpDisplayName, rpID string, origins []string) (*Service, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: rpDisplayName,
		RPID:          rpID,
		RPOrigins:     origins,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn: configuração inválida: %w", err)
	}
	return &Service{wa: wa}, nil
}

// User is the go-webauthn view of an identity and its registered WebAuthn
// factors. The WebAuthn user handle is the identity's OPAQUE subject (stable,
// non-personal) — never the e-mail or internal id.
type User struct {
	subject     string
	displayName string
	creds       []webauthn.Credential
}

// UserFromIdentity builds a WebAuthn user from the identity and its WebAuthn
// domain credentials (their PublicMaterial holds the serialized library
// credential). displayName is a non-personal label for the authenticator UI.
func UserFromIdentity(subject, displayName string, webauthnCreds []domain.Credential) (User, error) {
	u := User{subject: subject, displayName: displayName}
	for _, c := range webauthnCreds {
		if c.Type != domain.FactorWebAuthn {
			continue
		}
		var wc webauthn.Credential
		if err := json.Unmarshal(c.PublicMaterial, &wc); err != nil {
			return User{}, fmt.Errorf("webauthn: material da credencial %s inválido: %w", c.ID, err)
		}
		u.creds = append(u.creds, wc)
	}
	return u, nil
}

func (u User) WebAuthnID() []byte          { return []byte(u.subject) }
func (u User) WebAuthnName() string        { return u.subject }
func (u User) WebAuthnDisplayName() string { return u.displayName }
func (u User) WebAuthnIcon() string        { return "" }
func (u User) WebAuthnCredentials() []webauthn.Credential {
	return u.creds
}

// credentialExcludeList lists the user's existing credentials so registration
// does not enroll the same authenticator twice.
func (u User) credentialExcludeList() []protocol.CredentialDescriptor {
	out := make([]protocol.CredentialDescriptor, 0, len(u.creds))
	for _, c := range u.creds {
		out = append(out, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: c.ID,
		})
	}
	return out
}

// BeginRegistration starts enrolling a new authenticator, returning the options
// the client runs the ceremony with and the session data the caller MUST keep
// (the challenge) to pass to FinishRegistration.
func (s *Service) BeginRegistration(u User) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	return s.wa.BeginRegistration(u,
		webauthn.WithExclusions(u.credentialExcludeList()))
}

// FinishRegistration validates the authenticator's attestation response and
// returns the new factor as a domain.Credential — PublicMaterial only (INV-7),
// with its assurance level and non-secret metadata. session is the value from
// BeginRegistration; response is the raw attestation JSON body.
func (s *Service) FinishRegistration(identityID uuid.UUID, u User, session webauthn.SessionData, response io.Reader) (domain.Credential, error) {
	parsed, err := protocol.ParseCredentialCreationResponseBody(response)
	if err != nil {
		return domain.Credential{}, fmt.Errorf("webauthn: resposta de registro inválida: %w", err)
	}
	wc, err := s.wa.CreateCredential(u, session, parsed)
	if err != nil {
		return domain.Credential{}, fmt.Errorf("webauthn: registro recusado: %w", err)
	}
	return credentialToDomain(identityID, wc)
}

// BeginLogin starts an authentication ceremony against the user's registered
// authenticators.
func (s *Service) BeginLogin(u User) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	return s.wa.BeginLogin(u)
}

// LoginResult is the outcome of a completed authentication ceremony.
type LoginResult struct {
	// CredentialID identifies the authenticator that was used, so the caller can
	// match it to the stored domain credential (and read its assurance level).
	CredentialID []byte
	// SignCount is the authenticator's new signature counter — the caller
	// persists it to detect clones on the next login.
	SignCount uint32
	// CloneWarning is true when the signature counter went backwards, a signal
	// the authenticator may be cloned (WebAuthn spec §6.1.1). The caller decides
	// the risk response (deny, alert) — never silently ignores it.
	CloneWarning bool
}

// FinishLogin validates the authenticator's assertion and returns which
// credential was used plus the clone signal. session is the value from
// BeginLogin; response is the raw assertion JSON body.
func (s *Service) FinishLogin(u User, session webauthn.SessionData, response io.Reader) (LoginResult, error) {
	parsed, err := protocol.ParseCredentialRequestResponseBody(response)
	if err != nil {
		return LoginResult{}, fmt.Errorf("webauthn: resposta de autenticação inválida: %w", err)
	}
	wc, err := s.wa.ValidateLogin(u, session, parsed)
	if err != nil {
		return LoginResult{}, fmt.Errorf("webauthn: autenticação recusada: %w", err)
	}
	return LoginResult{
		CredentialID: wc.ID,
		SignCount:    wc.Authenticator.SignCount,
		CloneWarning: wc.Authenticator.CloneWarning,
	}, nil
}

// credentialToDomain serializes a verified WebAuthn credential into a
// domain.Credential (PublicMaterial only) and assigns its assurance level.
func credentialToDomain(identityID uuid.UUID, wc *webauthn.Credential) (domain.Credential, error) {
	material, err := json.Marshal(wc)
	if err != nil {
		return domain.Credential{}, fmt.Errorf("webauthn: serialização da credencial falhou: %w", err)
	}
	c, err := domain.NewWebAuthnCredential(identityID, material)
	if err != nil {
		return domain.Credential{}, err
	}
	if err := c.SetAssurance(assuranceFor(wc)); err != nil {
		return domain.Credential{}, err
	}
	c.Params = map[string]string{
		"credential_id":   base64.RawURLEncoding.EncodeToString(wc.ID),
		"aaguid":          base64.RawURLEncoding.EncodeToString(wc.Authenticator.AAGUID),
		"sign_count":      fmt.Sprintf("%d", wc.Authenticator.SignCount),
		"user_verified":   fmt.Sprintf("%t", wc.Flags.UserVerified),
		"backup_eligible": fmt.Sprintf("%t", wc.Flags.BackupEligible),
	}
	c.Label = "WebAuthn"
	return c, nil
}

// assuranceFor assigns the assurance level of a WebAuthn factor: AAL3 for a
// user-verified, NON-backup-eligible authenticator (a hardware key that cannot
// be synced/copied), AAL2 otherwise (a synced passkey, or one without user
// verification). This is the passkey-vs-hardware distinction ADR-0010 draws for
// privileged access.
func assuranceFor(wc *webauthn.Credential) domain.AAL {
	if wc.Flags.UserVerified && !wc.Flags.BackupEligible {
		return domain.AAL3
	}
	return domain.AAL2
}
