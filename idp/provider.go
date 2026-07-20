// Copyright 2021 The Casdoor Authors. All Rights Reserved.
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

package idp

import (
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

type UserInfo struct {
	Id          string
	Username    string
	DisplayName string
	UnionId     string
	Email       string
	Phone       string
	CountryCode string
	AvatarUrl   string
	Extra       map[string]string
}

type ProviderInfo struct {
	Type          string
	SubType       string
	ClientId      string
	ClientSecret  string
	ClientId2     string
	ClientSecret2 string
	AppId         string
	HostUrl       string
	RedirectUrl   string
	DisableSsl    bool
	CodeVerifier  string

	TokenURL    string
	AuthURL     string
	UserInfoURL string
	UserMapping map[string]string

	AppCertificate  string
	RootCertificate string
}

type IdProvider interface {
	SetHttpClient(client *http.Client)
	GetToken(code string) (*oauth2.Token, error)
	GetUserInfo(token *oauth2.Token) (*UserInfo, error)
}

func GetIdProvider(idpInfo *ProviderInfo, redirectUrl string) (IdProvider, error) {
	// Curated provider catalog (ADR-0015 §3): corporate IdPs + GitHub/GitLab +
	// generic Custom (OIDC/OAuth2). Non-curated social/regional providers and
	// the goth path were removed in T-010.
	switch idpInfo.Type {
	case "GitHub":
		return NewGithubIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "Google":
		return NewGoogleIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "GitLab":
		return NewGitlabIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "ADFS":
		return NewAdfsIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl), nil
	case "AzureADB2C":
		return NewAzureAdB2cProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl, idpInfo.AppId), nil
	case "Casdoor":
		return NewCasdoorIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl), nil
	case "Okta":
		return NewOktaIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl), nil
	case "Custom", "Custom Flexible":
		return NewCustomIdProvider(idpInfo, redirectUrl), nil
	default:
		if strings.HasPrefix(idpInfo.Type, "Custom") {
			return NewCustomIdProvider(idpInfo, redirectUrl), nil
		}
		return nil, fmt.Errorf("OAuth provider type: %s is not supported", idpInfo.Type)
	}
}
