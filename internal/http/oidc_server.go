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

package http

import "net/http"

// OIDCServer groups the ArchGuard OpenID Connect endpoints (pacote 006 wiring)
// into one http.Handler, ready to mount under the running router with a single
// line (e.g. web.Handler("/", server.Handler()) in Beego). Assembling the
// handlers with their dependencies (the keystore-backed signer, the pool, the
// client registry, the session resolver, the issuer config) happens at app init;
// this type is the mount point, keeping the routing in one auditable place.
type OIDCServer struct {
	Discovery  *DiscoveryHandler
	JWKS       *JWKSHandler
	Authorize  *AuthorizeHandler
	Token      *TokenHandler
	Introspect *IntrospectionHandler
	EndSession *EndSessionHandler
	PathPrefix string // e.g. "" or "/oidc"; endpoints are mounted under it
}

// Handler returns an http.Handler routing the OIDC endpoints. Nil handlers are
// simply not mounted (a partial wiring), so the mux never dispatches to a nil
// handler. The paths follow the discovery document's endpoint URLs.
func (s *OIDCServer) Handler() http.Handler {
	mux := http.NewServeMux()
	p := s.PathPrefix
	mount := func(path string, h http.Handler) {
		if h != nil {
			mux.Handle(p+path, h)
		}
	}
	if s.Discovery != nil {
		// Discovery lives at the well-known path, NOT under the prefix.
		mux.Handle("/.well-known/openid-configuration", s.Discovery)
	}
	mount("/jwks", s.JWKS)
	mount("/authorize", s.Authorize)
	mount("/token", s.Token)
	mount("/introspect", s.Introspect)
	mount("/logout", s.EndSession)
	return mux
}
