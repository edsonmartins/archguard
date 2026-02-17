// src/components/oauth2/integration-snippets.ts

import type { OAuth2Client } from '@/lib/api/types/kanidm'

interface Snippet {
  framework: string
  code: string
}

export function getIntegrationSnippets(client: OAuth2Client): Snippet[] {
  const issuer = typeof window !== 'undefined'
    ? `${window.location.origin}/api/id`
    : 'https://id.archguard.local'

  return [
    {
      framework: 'React (oidc-client-ts)',
      code: `import { UserManager } from 'oidc-client-ts';

const userManager = new UserManager({
  authority: '${issuer}/oauth2/openid/${client.name}',
  client_id: '${client.name}',
  redirect_uri: '${client.redirectUrls[0] ?? 'http://localhost:3000/callback'}',
  response_type: 'code',
  scope: 'openid profile email',${client.type === 'public' ? "\n  // Public client - no client_secret needed" : ''}
});

// Login
userManager.signinRedirect();

// Handle callback
const user = await userManager.signinCallback();
console.log('Token:', user.access_token);`,
    },
    {
      framework: 'Spring Boot (application.yml)',
      code: `spring:
  security:
    oauth2:
      client:
        registration:
          ${client.name}:
            client-id: ${client.name}${client.type === 'basic' ? '\n            client-secret: <SECRET>' : ''}
            scope: openid,profile,email
            authorization-grant-type: authorization_code
            redirect-uri: "{baseUrl}/login/oauth2/code/${client.name}"
        provider:
          ${client.name}:
            issuer-uri: ${issuer}/oauth2/openid/${client.name}`,
    },
    {
      framework: 'Node.js (openid-client)',
      code: `import { Issuer } from 'openid-client';

const issuer = await Issuer.discover(
  '${issuer}/oauth2/openid/${client.name}'
);

const client = new issuer.Client({
  client_id: '${client.name}',${client.type === 'basic' ? "\n  client_secret: '<SECRET>'," : ''}
  redirect_uris: ['${client.redirectUrls[0] ?? 'http://localhost:3000/callback'}'],
  response_types: ['code'],
});

// Generate auth URL
const authUrl = client.authorizationUrl({
  scope: 'openid profile email',
});`,
    },
    {
      framework: 'Python (authlib)',
      code: `from authlib.integrations.requests_client import OAuth2Session

client = OAuth2Session(
    client_id='${client.name}',${client.type === 'basic' ? "\n    client_secret='<SECRET>'," : ''}
    scope='openid profile email',
    redirect_uri='${client.redirectUrls[0] ?? 'http://localhost:3000/callback'}',
    code_challenge_method='S256',
)

# Generate auth URL
uri, state = client.create_authorization_url(
    '${issuer}/oauth2/openid/${client.name}/authorize'
)`,
    },
    {
      framework: 'Flutter (flutter_appauth)',
      code: `import 'package:flutter_appauth/flutter_appauth.dart';

final appAuth = FlutterAppAuth();

final result = await appAuth.authorizeAndExchangeCode(
  AuthorizationTokenRequest(
    '${client.name}',
    '${client.redirectUrls[0] ?? 'com.example.app://callback'}',
    issuer: '${issuer}/oauth2/openid/${client.name}',
    scopes: ['openid', 'profile', 'email'],
  ),
);

print('Access token: \${result?.accessToken}');`,
    },
  ]
}
