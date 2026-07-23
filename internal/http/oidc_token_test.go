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

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
)

type fakeRefreshGrant struct {
	result domain.RefreshResult
	err    error
}

func (f fakeRefreshGrant) Refresh(context.Context, string) (domain.RefreshResult, error) {
	return f.result, f.err
}

func postToken(t *testing.T, h *TokenHandler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Renovação normal: retorna access + refresh novos.
func TestTokenHandlerRefreshOK(t *testing.T) {
	h := NewTokenHandler(fakeRefreshGrant{result: domain.RefreshResult{
		AccessToken: "access-jwt", RefreshToken: "rt_new", ExpiresInSecond: 600,
	}})
	rec := postToken(t, h, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"rt_old"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, quero 200", rec.Code)
	}
	var body tokenSuccessBody
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.AccessToken != "access-jwt" || body.RefreshToken != "rt_new" || body.TokenType != "Bearer" || body.ExpiresIn != 600 {
		t.Fatalf("resposta de token inesperada: %+v", body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("a resposta de token deveria ser no-store")
	}
}

// Reuso detectado: invalid_grant (a família já foi revogada no adapter).
func TestTokenHandlerRefreshReuse(t *testing.T) {
	h := NewTokenHandler(fakeRefreshGrant{err: domain.ErrRefreshReuse})
	rec := postToken(t, h, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"rt_reused"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, quero 400", rec.Code)
	}
	var body tokenErrorBody
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Error != "invalid_grant" {
		t.Fatalf("erro = %q, quero invalid_grant", body.Error)
	}
}

// ROPC e grants não suportados são recusados.
func TestTokenHandlerRejectsUnsupportedGrant(t *testing.T) {
	h := NewTokenHandler(fakeRefreshGrant{})
	rec := postToken(t, h, url.Values{"grant_type": {"password"}, "username": {"x"}, "password": {"y"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ROPC deveria ser recusado com 400, veio %d", rec.Code)
	}
	var body tokenErrorBody
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Error != "unsupported_grant_type" {
		t.Fatalf("erro = %q, quero unsupported_grant_type", body.Error)
	}
}
