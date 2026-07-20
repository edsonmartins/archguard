// Copyright 2023 The Casdoor Authors. All Rights Reserved.
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

package notification

import notify "github.com/casdoor/notify2"

// GetNotificationProvider returns a notifier for the curated set (ADR-0015 §2/§3):
// webhook (Custom HTTP) plus the corporate channels. Non-curated consumer and
// regional providers were removed in T-010.
func GetNotificationProvider(typ string, clientId string, clientSecret string, clientId2 string, clientSecret2 string, appId string, receiver string, method string, title string, metaData string, regionId string) (notify.Notifier, error) {
	if typ == "Custom HTTP" {
		return NewCustomHttpProvider(receiver, method, title)
	} else if typ == "Microsoft Teams" {
		return NewMicrosoftTeamsProvider(clientSecret)
	} else if typ == "Slack" {
		return NewSlackProvider(clientSecret, receiver)
	} else if typ == "Google Chat" {
		return NewGoogleChatProvider(metaData)
	}

	return nil, nil
}
