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

package domain

import (
	"errors"
	"fmt"
)

// ErrUnknownClient is returned when a client id is not in the registry.
var ErrUnknownClient = errors.New("oidc: cliente não registrado")

// ErrFlowNotAllowedForClient is returned when a client attempts a flow it is not
// registered for.
var ErrFlowNotAllowedForClient = errors.New("oidc: fluxo não permitido para este cliente")

// OIDCClient is a registered ArchGate component (RFC-0006 §2). Each has its OWN
// audience and the MINIMAL flows and scopes it needs; a dedicated audience per
// component is what makes a token for one component unusable by another (ADR-0011).
type OIDCClient struct {
	ClientID      string
	Audience      string
	AllowedFlows  []OAuthFlow
	AllowedScopes []string
	RedirectURIs  []string
	// BackchannelLogoutURI is the endpoint that receives back-channel logout
	// tokens; empty when the component has no support (it relies on short-TTL
	// introspection instead, RFC-0006 §6).
	BackchannelLogoutURI string
	// Notes documents component-specific handling (edge adaptation, device flow).
	Notes string
}

// AllowsFlow reports whether the client is registered for flow.
func (c OIDCClient) AllowsFlow(flow OAuthFlow) bool {
	for _, f := range c.AllowedFlows {
		if f == flow {
			return true
		}
	}
	return false
}

// SupportsBackchannelLogout reports whether the client has a back-channel logout
// endpoint (else revocation reaches it via introspection).
func (c OIDCClient) SupportsBackchannelLogout() bool {
	return c.BackchannelLogoutURI != ""
}

// ClientRegistry holds the registered components, keyed by client id.
type ClientRegistry struct {
	clients map[string]OIDCClient
}

// NewClientRegistry builds an empty registry.
func NewClientRegistry() *ClientRegistry {
	return &ClientRegistry{clients: make(map[string]OIDCClient)}
}

// Register adds a client, refusing an empty id or audience or a duplicate id.
func (r *ClientRegistry) Register(c OIDCClient) error {
	if c.ClientID == "" || c.Audience == "" {
		return fmt.Errorf("%w: client_id/audience obrigatórios", ErrUnknownClient)
	}
	if len(c.AllowedFlows) == 0 {
		return fmt.Errorf("%w: cliente %q sem fluxo permitido", ErrUnknownClient, c.ClientID)
	}
	if _, exists := r.clients[c.ClientID]; exists {
		return fmt.Errorf("oidc: cliente %q já registrado", c.ClientID)
	}
	r.clients[c.ClientID] = c
	return nil
}

// Lookup returns a client by id, or ErrUnknownClient.
func (r *ClientRegistry) Lookup(clientID string) (OIDCClient, error) {
	c, ok := r.clients[clientID]
	if !ok {
		return OIDCClient{}, fmt.Errorf("%w: %q", ErrUnknownClient, clientID)
	}
	return c, nil
}

// IDs returns the registered client ids (unordered).
func (r *ClientRegistry) IDs() []string {
	out := make([]string, 0, len(r.clients))
	for id := range r.clients {
		out = append(out, id)
	}
	return out
}

// AuthorizeClientFlow validates that a client may use a flow — the gate the
// authorization/token endpoint runs before proceeding.
func (r *ClientRegistry) AuthorizeClientFlow(clientID string, flow OAuthFlow) (OIDCClient, error) {
	c, err := r.Lookup(clientID)
	if err != nil {
		return OIDCClient{}, err
	}
	if !c.AllowsFlow(flow) {
		return OIDCClient{}, fmt.Errorf("%w: %q não permite %s", ErrFlowNotAllowedForClient, clientID, flow)
	}
	return c, nil
}

// DefaultClientRegistry registers the ArchGate components with the profiles of
// RFC-0006 §2. Warpgate and NetBird use Authorization Code + PKCE (NetBird also
// the device flow, for browserless clients); Guacamole uses Authorization Code
// with edge adaptation and no reliable logout (short-TTL introspection); OpenBao
// and the Oracle proxy only VALIDATE JWTs (no interactive flow of their own) — but
// they still need a registered audience so a token minted for them is bound.
func DefaultClientRegistry() (*ClientRegistry, error) {
	reg := NewClientRegistry()
	clients := []OIDCClient{
		{
			ClientID:             "warpgate",
			Audience:             "warpgate",
			AllowedFlows:         []OAuthFlow{FlowAuthorizationCode},
			AllowedScopes:        []string{"openid", "profile"},
			RedirectURIs:         []string{"https://warpgate.archgate.internal/@warpgate/oidc/callback"},
			BackchannelLogoutURI: "https://warpgate.archgate.internal/@warpgate/oidc/logout",
			Notes:                "SSO web e sessões de bastião; Authorization Code + PKCE.",
		},
		{
			ClientID:      "guacamole",
			Audience:      "guacamole",
			AllowedFlows:  []OAuthFlow{FlowAuthorizationCode},
			AllowedScopes: []string{"openid", "profile"},
			RedirectURIs:  []string{"https://guacamole.archgate.internal/guacamole/"},
			// Sem back-channel logout confiável: revogação via introspecção de TTL
			// curto; adaptação de borda no próprio Guacamole (T-015).
			Notes: "Extensão OIDC com suporte limitado; adaptação de borda; introspecção de TTL curto.",
		},
		{
			ClientID:             "netbird",
			Audience:             "netbird",
			AllowedFlows:         []OAuthFlow{FlowAuthorizationCode, FlowDeviceCode},
			AllowedScopes:        []string{"openid", "profile", "offline_access"},
			RedirectURIs:         []string{"https://netbird.archgate.internal/peers/callback"},
			BackchannelLogoutURI: "https://netbird.archgate.internal/oidc/logout",
			Notes:                "Authorization Code + PKCE; Device Authorization Grant para clientes sem navegador (sem L3).",
		},
		{
			ClientID:      "openbao",
			Audience:      "openbao",
			AllowedFlows:  []OAuthFlow{FlowAuthorizationCode},
			AllowedScopes: []string{"openid"},
			RedirectURIs:  []string{"https://openbao.archgate.internal/ui/vault/auth/oidc/oidc/callback"},
			Notes:         "Auth method JWT/OIDC; mapeamento de claims → políticas do cofre (T-014).",
		},
		{
			ClientID:      "oracle-jdbc-proxy",
			Audience:      "oracle-jdbc-proxy",
			AllowedFlows:  []OAuthFlow{FlowAuthorizationCode},
			AllowedScopes: []string{"openid"},
			// Sem fluxo interativo próprio: valida JWT via JWKS. A audiência dedicada
			// vincula o token ao proxy.
			Notes: "Validação de JWT (JWKS); sem fluxo interativo próprio.",
		},
	}
	for _, c := range clients {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return reg, nil
}
