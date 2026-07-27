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

package boot

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/adapters/webauthn"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// webauthnChallengeTTL bounds how long a begun assertion challenge is valid — the
// ceremony is a couple of round-trips, so a short window is safe and limits replay.
const webauthnChallengeTTL = 5 * time.Minute

// webauthnChallenges keeps the per-session assertion challenge (go-webauthn SessionData)
// between begin and finish. It is IN-MEMORY, keyed by session id, one-shot and TTL-bounded.
// CAVEAT (pilot, single-replica): a challenge does not survive a process restart nor a
// second replica — the caller simply retries begin. Migrate to a shared/DB store when the
// control plane scales horizontally.
type webauthnChallenges struct {
	mu   sync.Mutex
	data map[string]webauthnChallenge
}

type webauthnChallenge struct {
	session gowebauthn.SessionData
	expires time.Time
}

func newWebauthnChallenges() *webauthnChallenges {
	return &webauthnChallenges{data: map[string]webauthnChallenge{}}
}

func (c *webauthnChallenges) put(sessionID string, sd gowebauthn.SessionData, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[sessionID] = webauthnChallenge{session: sd, expires: now.Add(webauthnChallengeTTL)}
}

// take returns the challenge and removes it (one-shot). A missing or expired challenge
// returns ok=false — the finish is then a denial (fail-closed).
func (c *webauthnChallenges) take(sessionID string, now time.Time) (gowebauthn.SessionData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch, ok := c.data[sessionID]
	if !ok {
		return gowebauthn.SessionData{}, false
	}
	delete(c.data, sessionID)
	if now.After(ch.expires) {
		return gowebauthn.SessionData{}, false
	}
	return ch.session, true
}

// webauthnStepUp raises a live session to the phishing-resistant AAL of a WebAuthn
// authenticator after a valid assertion — the only step-up that satisfies L3 (TOTP caps at
// AAL2). It runs the go-webauthn assertion ceremony, matches the used credential to read
// its assurance level, then applies domain StepUp + persists it in one identity tx.
type webauthnStepUp struct {
	svc        *webauthn.Service
	creds      *postgres.CredentialStore
	identities *postgres.IdentityStore
	pool       *pgxpool.Pool
	challenges *webauthnChallenges
}

// webauthnServiceFromConf builds the relying party from the deployment origin config. The
// RP ID is the origin's host (stable, no scheme/port); the permitted origins are the
// configured front/back origins. In production `origin` MUST be the console's HTTPS URL
// (e.g. https://app.archguard.com.br), else the browser rejects the ceremony.
func webauthnServiceFromConf() (*webauthn.Service, error) {
	origins := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, key := range []string{"origin", "originFrontend"} {
		o := strings.TrimRight(conf.GetConfigString(key), "/")
		if o != "" && !seen[o] {
			origins = append(origins, o)
			seen[o] = true
		}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:7001"} // dev default
	}
	rpID := "localhost"
	if u, err := url.Parse(origins[0]); err == nil && u.Hostname() != "" {
		rpID = u.Hostname()
	}
	return webauthn.NewService("ArchGuard", rpID, origins)
}

// newWebauthnStepUp composes the WebAuthn step-up service from the factory.
func newWebauthnStepUp(f *Factory) (*webauthnStepUp, error) {
	svc, err := webauthnServiceFromConf()
	if err != nil {
		return nil, err
	}
	return &webauthnStepUp{
		svc:        svc,
		creds:      postgres.NewCredentialStore(f.Pool()),
		identities: postgres.NewIdentityStore(f.Pool()),
		pool:       f.Pool(),
		challenges: newWebauthnChallenges(),
	}, nil
}

// userAndCreds loads the identity's credentials and builds the go-webauthn user over its
// WebAuthn factors. A caller with no WebAuthn factor cannot step up this way
// (ErrNoStrongFactor → 409).
func (s *webauthnStepUp) userAndCreds(ctx context.Context, identityID uuid.UUID) (webauthn.User, []domain.Credential, error) {
	creds, err := s.creds.ListByIdentity(ctx, identityID)
	if err != nil {
		return webauthn.User{}, nil, err
	}
	hasWebAuthn := false
	for i := range creds {
		if creds[i].Type == domain.FactorWebAuthn {
			hasWebAuthn = true
			break
		}
	}
	if !hasWebAuthn {
		return webauthn.User{}, nil, apihttp.ErrNoStrongFactor
	}
	idn, err := s.identities.Get(ctx, identityID)
	if err != nil {
		return webauthn.User{}, nil, err
	}
	u, err := webauthn.UserFromIdentity(idn.Subject, "ArchGuard", creds)
	if err != nil {
		return webauthn.User{}, nil, err
	}
	return u, creds, nil
}

// BeginWebAuthn starts the assertion ceremony and stashes the challenge for finish.
func (s *webauthnStepUp) BeginWebAuthn(ctx context.Context, identityID, sessionID uuid.UUID) (any, error) {
	u, _, err := s.userAndCreds(ctx, identityID)
	if err != nil {
		return nil, err
	}
	options, sessionData, err := s.svc.BeginLogin(u)
	if err != nil {
		return nil, err
	}
	s.challenges.put(sessionID.String(), *sessionData, time.Now())
	return options, nil
}

// FinishWebAuthn validates the assertion against the stashed challenge and, on success,
// raises the session to the used authenticator's AAL (phishing-resistant by construction).
func (s *webauthnStepUp) FinishWebAuthn(ctx context.Context, identityID, sessionID uuid.UUID, assertion []byte, now time.Time) (domain.AAL, error) {
	sessionData, ok := s.challenges.take(sessionID.String(), now)
	if !ok {
		return "", apihttp.ErrStepUpDenied // no/expired challenge — a denial
	}
	u, creds, err := s.userAndCreds(ctx, identityID)
	if err != nil {
		return "", err
	}
	result, err := s.svc.FinishLogin(u, sessionData, bytes.NewReader(assertion))
	if err != nil {
		return "", apihttp.ErrStepUpDenied // invalid assertion — a denial, not an error
	}

	// The proven AAL is that of the authenticator actually used (hardware AAL3 vs synced
	// passkey AAL2). WebAuthn is phishing-resistant regardless, so AAL2 is the safe floor.
	aal := domain.AAL2
	usedID := base64.RawURLEncoding.EncodeToString(result.CredentialID)
	for i := range creds {
		if creds[i].Type == domain.FactorWebAuthn && creds[i].Params["credential_id"] == usedID {
			if creds[i].AAL != "" {
				aal = creds[i].AAL
			}
			break
		}
	}

	scope, err := domain.NewIdentityScope(identityID)
	if err != nil {
		return "", err
	}
	var newAAL domain.AAL
	err = postgres.NewIdentityRepository(s.pool, scope).WithIdentityTx(ctx, func(itx *postgres.IdentityTx) error {
		store := postgres.NewIdentitySessionStore(itx)
		session, gerr := store.Get(ctx, sessionID)
		if gerr != nil {
			return gerr
		}
		if serr := session.StepUp(now, aal, []domain.FactorType{domain.FactorWebAuthn}); serr != nil {
			return serr
		}
		if serr := store.SaveStepUp(ctx, session); serr != nil {
			return serr
		}
		newAAL = session.ProvenAAL
		return nil
	})
	if err != nil {
		return "", err
	}
	return newAAL, nil
}
