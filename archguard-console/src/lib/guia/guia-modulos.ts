// Guia contextual por rota — padrão RecomX (Guia do Módulo).
// ArchGate Manager: um console (estilo AWS); o guia explica tela + negócio.

export type ItemFluxo = {
  passo: number
  acao: string
  descricao: string
}

export type GuiaModulo = {
  /** Pathname canônico, ex. /org-accounts */
  rota: string
  icone: string
  titulo: string
  subtitulo: string
  modulo: string
  fase: string
  corAccent: string
  objetivo: string
  problema: string
  comoFunciona: string[]
  fluxoSugerido: ItemFluxo[]
  regrasChave: string[]
  integracoesFuturas?: string[]
}

export const guiaModulos: GuiaModulo[] = [
  {
    rota: '/dashboard',
    icone: '🏠',
    titulo: 'Dashboard',
    subtitulo: 'Visão geral do ArchGate Manager',
    modulo: 'AG-HOME',
    fase: 'Core',
    corAccent: '#2563eb',
    objetivo:
      'Ponto de entrada do console: saúde da plataforma, atalhos para módulos e contadores de identidade. Configure o portão daqui — sessões de operador ficam no Connect.',
    problema:
      'Sem um painel único, o admin se perde entre Kanidm, OpenBao e Warpgate. O Manager concentra o estado e o próximo passo.',
    comoFunciona: [
      'KPIs de pessoas, grupos e clients OAuth2 vêm do módulo de identidade (Kanidm via BFF)',
      'Saúde OpenBao e Kanidm aparece nos cards de sistema',
      'Atalhos levam aos módulos de Acesso (sites, gateways, contas da org)',
      'Tudo passa por SSO; permissões vêm dos grupos Kanidm',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Confira a saúde',
        descricao: 'Veja se identidade e vault estão online antes de operar.',
      },
      {
        passo: 2,
        acao: 'Revise identidades',
        descricao: 'Cadastre ou revogue pessoas se houver onboarding/offboarding.',
      },
      {
        passo: 3,
        acao: 'Sites e gateways',
        descricao: 'Mantenha clientes (sites) e targets do bastion alinhados.',
      },
      {
        passo: 4,
        acao: 'Contas da org',
        descricao: 'Checkout de credenciais corporativas (lojas/produtos) com dual-control.',
      },
    ],
    regrasChave: [
      'Manager configura; Connect/UnifiedUI operam sessões de bastion',
      'Token OpenBao nunca vai ao browser',
      'RBAC deny-by-default por permissão de grupo',
    ],
  },
  {
    rota: '/identities',
    icone: '👤',
    titulo: 'Identidades',
    subtitulo: 'Pessoas no IdP (Kanidm)',
    modulo: 'AG-ID',
    fase: 'Identidade',
    corAccent: '#7c3aed',
    objetivo:
      'Gerenciar pessoas: criar, editar, grupos, reset de credencial e offboarding com um clique.',
    problema:
      'Sem identidade central, cada sistema tem usuário órfão. Offboarding fraco deixa acesso residual.',
    comoFunciona: [
      'Lista e detalhe via API Kanidm (proxy server-side)',
      'Wizard de criação e import CSV',
      'Revogar acesso expira conta no IdP e fecha checkouts de contas da org',
      'Filtro por tenant conforme grupos do admin',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Busque a pessoa',
        descricao: 'Filtre por nome/e-mail antes de criar duplicata.',
      },
      {
        passo: 2,
        acao: 'Ajuste grupos',
        descricao: 'Grupos definem permissões do Manager e apps OIDC.',
      },
      {
        passo: 3,
        acao: 'Offboarding',
        descricao: 'Use Revogar acesso — não delete à toa (auditoria).',
      },
    ],
    regrasChave: [
      'Expire (account_expire) é preferível a hard delete',
      'Operadores de bastion precisam de grupo + mapping Warpgate',
      'SSO no Manager usa o client archguard-console',
    ],
  },
  {
    rota: '/service-accounts',
    icone: '🤖',
    titulo: 'Service Accounts',
    subtitulo: 'Contas de máquina e tokens',
    modulo: 'AG-SA',
    fase: 'Identidade',
    corAccent: '#0891b2',
    objetivo:
      'Provisionar service accounts e tokens de API para automações, sem usuário humano compartilhado.',
    problema:
      'Scripts com senha de pessoa quebram offboarding e geram auditoria confusa.',
    comoFunciona: [
      'CRUD de SA no IdP',
      'Geração e revogação de tokens com rótulo',
      'Status active/expired/disabled',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Crie a SA',
        descricao: 'Nome claro do sistema consumidor (ex. ci-archgate).',
      },
      {
        passo: 2,
        acao: 'Gere o token',
        descricao: 'Copie uma vez; guarde no secret store do pipeline.',
      },
      {
        passo: 3,
        acao: 'Revogue se vazou',
        descricao: 'Revogação imediata no detalhe da SA.',
      },
    ],
    regrasChave: [
      'Token exibido uma vez na geração',
      'Prefira SA a senha de pessoa em CI',
    ],
  },
  {
    rota: '/groups',
    icone: '👥',
    titulo: 'Grupos',
    subtitulo: 'RBAC e tenants',
    modulo: 'AG-GRP',
    fase: 'Identidade',
    corAccent: '#4f46e5',
    objetivo:
      'Organizar membros em grupos que viram permissões no Manager (sites, org_accounts, vault…).',
    problema:
      'Sem grupos, cada permissão vira exceção manual e offboarding falha.',
    comoFunciona: [
      'Grupos Kanidm com membros',
      'derivePermissions mapeia grupo → permission strings',
      'Grupos tenant_* isolam sites por cliente',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Liste grupos de plataforma',
        descricao: 'archguard_users, super_admins, viewers…',
      },
      {
        passo: 2,
        acao: 'Ajuste membros',
        descricao: 'No detalhe do grupo, adicione/remova pessoas.',
      },
      {
        passo: 3,
        acao: 'Tenants',
        descricao: 'Grupos tenant_* amarram o admin aos sites do cliente.',
      },
    ],
    regrasChave: [
      'system:admin bypassa; demais são deny-by-default',
      'org_accounts:* controla o broker de credenciais da casa',
    ],
  },
  {
    rota: '/oauth2',
    icone: '🔑',
    titulo: 'OAuth2 / SSO',
    subtitulo: 'Clients OIDC no IdP',
    modulo: 'AG-OIDC',
    fase: 'Identidade',
    corAccent: '#db2777',
    objetivo:
      'Registrar aplicações (Manager, produtos, Connect) como clients OAuth2/OIDC no Kanidm — tudo pelo console.',
    problema:
      'Cada produto com login local vira senha compartilhada e offboarding impossível.',
    comoFunciona: [
      'Wizard de criação de client (redirect URIs, PKCE)',
      'Detalhe: secrets, redirects, matriz de acesso',
      'Produtos da org usam client_id referenciado em Contas da org',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Crie o client',
        descricao: 'Redirect URIs exatos do admin do produto.',
      },
      {
        passo: 2,
        acao: 'Scopes e grupos',
        descricao: 'openid profile email groups.',
      },
      {
        passo: 3,
        acao: 'Atualize inventário',
        descricao: 'Em Contas da org, federation_status + oidc_client_id.',
      },
    ],
    regrasChave: [
      'Não compartilhe client_secret no chat',
      'PKCE para clients públicos',
      'Implementação do login no produto é código do produto (4.4)',
    ],
    integracoesFuturas: ['Vendax', 'ArchFlow', 'Gestor', 'BrainSentry'],
  },
  {
    rota: '/vault',
    icone: '🔐',
    titulo: 'Vault',
    subtitulo: 'Saúde e operações do backend de segredos',
    modulo: 'AG-VAULT',
    fase: 'Segredos',
    corAccent: '#ca8a04',
    objetivo:
      'Monitorar o OpenBao (init, seal, mounts) pelo Manager. Uso avançado de plataforma — o dia a dia de contas org usa Contas da org.',
    problema:
      'Vault selado ou sem token quebra checkout e store secret em silêncio.',
    comoFunciona: [
      'Status sealed/unsealed e versão',
      'Listagem de mounts (admin)',
      'Unseal com chave de bootstrap quando permitido',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Confira sealed',
        descricao: 'Se selado, só admin de plataforma unseal.',
      },
      {
        passo: 2,
        acao: 'Contas da org',
        descricao: 'Grave secrets de negócio em Contas da org, não aqui.',
      },
    ],
    regrasChave: [
      'Root token bloqueado em produção salvo emergência',
      'Operador de contas usa Contas da org + health do broker',
    ],
  },
  {
    rota: '/sites',
    icone: '🏢',
    titulo: 'Clientes / Sites',
    subtitulo: 'Inventário de clientes e conectividade',
    modulo: 'AG-SITE',
    fase: 'Acesso',
    corAccent: '#059669',
    objetivo:
      'Cadastrar cada cliente (site): tenant, connector, subnets, targets e checklist de deploy do agente.',
    problema:
      'Sem inventário de sites, o bastion vira lista solta de targets sem dono nem tenant.',
    comoFunciona: [
      'CRUD + YAML import/export',
      'Wizard de onboarding de site',
      'Targets e secret_ref de host',
      'Painel de status do connector',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Crie ou wizard',
        descricao: 'Defina tenant_group e tipo de stack.',
      },
      {
        passo: 2,
        acao: 'Connector',
        descricao: 'Deploy/probe do agente no ambiente do cliente.',
      },
      {
        passo: 3,
        acao: 'Targets',
        descricao: 'Publique no Warpgate via Gateways se necessário.',
      },
    ],
    regrasChave: [
      'Tenant isolation: admin só vê sites dos seus tenant_*',
      'Segredos de host no OpenBao paths de customer, não org/*',
    ],
  },
  {
    rota: '/gateways',
    icone: '🚪',
    titulo: 'Gateways',
    subtitulo: 'Warpgate, Guacamole e sessões',
    modulo: 'AG-GW',
    fase: 'Acesso',
    corAccent: '#0d9488',
    objetivo:
      'Operar o plano de acesso privilegiado: targets Warpgate, roles, sessões e conexões Guacamole.',
    problema:
      'Sem gateway unificado, cada bastion é um silo e o Connect não tem catálogo.',
    comoFunciona: [
      'Painéis Warpgate (targets, roles, sessões)',
      'Guacamole (RDP/VNC/DB browser)',
      'Apply de targets a partir de sites',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Revise targets',
        descricao: 'Confira hosts e roles após mudança de site.',
      },
      {
        passo: 2,
        acao: 'Sessões ativas',
        descricao: 'Encerre sessão órfã se necessário.',
      },
      {
        passo: 3,
        acao: 'Smoke Connect',
        descricao: 'Operador valida catálogo no desktop Connect.',
      },
    ],
    regrasChave: [
      'Manager não embute RDP/SSH do dia a dia (Connect)',
      'Credenciais de target via secret_ref no backend',
    ],
  },
  {
    rota: '/secrets',
    icone: '🗝️',
    titulo: 'Segredos',
    subtitulo: 'Navegação e escrita assistida de secrets',
    modulo: 'AG-SEC',
    fase: 'Segredos',
    corAccent: '#b45309',
    objetivo:
      'Ferramentas de secrets de plataforma/targets. Para contas IntegrAllTech (lojas/produtos), prefira Contas da org.',
    problema:
      'Misturar secret de cliente e senha da App Store no mesmo fluxo confunde policy e audit.',
    comoFunciona: [
      'Browse paths permitidos ao app token',
      'Store de secrets de target quando aplicável',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Identifique o bolso',
        descricao: 'customer/* vs org/* vs ci/*.',
      },
      {
        passo: 2,
        acao: 'Contas da org',
        descricao: 'Apple/Play/GCP/admins de produto → módulo dedicado.',
      },
    ],
    regrasChave: [
      'Nunca logar valor de secret na auditoria',
      'Caminhos org/* têm dual-control no checkout',
    ],
  },
  {
    rota: '/org-accounts',
    icone: '🏦',
    titulo: 'Contas da org',
    subtitulo: 'Org Credential Broker (ADR-013)',
    modulo: 'AG-OCB',
    fase: 'Acesso',
    corAccent: '#dc2626',
    objetivo:
      'Inventário e checkout controlado de contas da IntegrAllTech (GCP, lojas, admins de produto). Console-only: sem UI do vault/IdP no dia a dia.',
    problema:
      'Senhas da casa no chat/planilha/1Password pessoal — offboarding e audit impossíveis.',
    comoFunciona: [
      'Inventário com federação e secret_ref (metadados)',
      'Gravar secret e checkout via BFF → backend de segredos',
      'P0 exige dual-control (segundo aprovador)',
      'Status do broker + configurações (webhook, TTL) nesta tela',
      'Connect só deep-link — sem vault local',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Veja o status',
        descricao: 'Card “Status do broker” deve estar Operacional / Secrets OK.',
      },
      {
        passo: 2,
        acao: 'Grave o secret',
        descricao: 'Cadeado na linha — path criado se vazio.',
      },
      {
        passo: 3,
        acao: 'Checkout',
        descricao: 'Motivo + TTL; P0 vai para fila de aprovação.',
      },
      {
        passo: 4,
        acao: 'Check-in',
        descricao: 'Encerre a janela ao terminar; não cole no Slack.',
      },
    ],
    regrasChave: [
      'Token do backend de segredos nunca no browser',
      'Self-approve de P0 é proibido',
      'Preferir OIDC/API key a senha compartilhada (federation_status)',
      'Webhook e TTL em Configurações do broker (não SSH)',
    ],
  },
  {
    rota: '/oracle',
    icone: '🗄️',
    titulo: 'Oracle',
    subtitulo: 'Acesso e credenciais Oracle',
    modulo: 'AG-ORA',
    fase: 'Acesso',
    corAccent: '#c026d3',
    objetivo:
      'Operações assistidas para bancos Oracle no contexto ArchGate (UI/credenciais via backend).',
    problema:
      'Credenciais Oracle espalhadas geram risco e dificultam bastion-first.',
    comoFunciona: [
      'Fluxos de provisionamento/consulta definidos no módulo',
      'Pode gravar material no backend de segredos quando marcado',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Identifique o target',
        descricao: 'Use site/gateway alinhados ao cliente.',
      },
      {
        passo: 2,
        acao: 'Prefira bastion',
        descricao: 'Acesso via Warpgate/Connect quando possível.',
      },
    ],
    regrasChave: ['Não reutilize senha de pessoa para jobs Oracle'],
  },
  {
    rota: '/platform',
    icone: '⚙️',
    titulo: 'Plataforma',
    subtitulo: 'Overview do control plane',
    modulo: 'AG-PLT',
    fase: 'Core',
    corAccent: '#475569',
    objetivo:
      'Visão consolidada de componentes da plataforma para super-admins.',
    problema:
      'Incidentes de plataforma exigem um lugar para ver o “estado do mundo”.',
    comoFunciona: [
      'Agrega status de integrações e serviços',
      'Leitura privilegiada (settings:read / system:admin)',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Leia o overview',
        descricao: 'Identifique serviço degradado.',
      },
      {
        passo: 2,
        acao: 'Vá ao módulo',
        descricao: 'Vault, Gateways ou Identidades conforme o sintoma.',
      },
    ],
    regrasChave: ['Mudanças de bootstrap continuam em devops; UI é operação'],
  },
  {
    rota: '/integrations/mentors-axis',
    icone: '☁️',
    titulo: 'Mentors Axis',
    subtitulo: 'Integração externa',
    modulo: 'AG-AXIS',
    fase: 'Integrações',
    corAccent: '#0284c7',
    objetivo:
      'Status e listagens da integração Mentors Axis a partir do Manager.',
    problema:
      'Integrações sem painel ficam só em log de API.',
    comoFunciona: [
      'Proxy/status server-side',
      'Listagens de entidades expostas pela integração',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Confira status',
        descricao: 'Se offline, verifique rede/credencial da integração.',
      },
    ],
    regrasChave: ['Credenciais da integração não ficam no browser'],
  },
  {
    rota: '/audit',
    icone: '📋',
    titulo: 'Auditoria',
    subtitulo: 'Trilha de ações do console',
    modulo: 'AG-AUD',
    fase: 'Governança',
    corAccent: '#64748b',
    objetivo:
      'Consultar quem fez o quê no Manager (incluindo checkout de contas da org).',
    problema:
      'Sem audit, dual-control e compliance não se sustentam.',
    comoFunciona: [
      'Activity log local (SQLite) + filtros',
      'Eventos de org-accounts sem valores de secret',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Filtre por ator ou path',
        descricao: 'Ex.: /archgate/org-accounts',
      },
      {
        passo: 2,
        acao: 'Investigue incidente',
        descricao: 'Correlacione com offboarding e rotação de secret.',
      },
    ],
    regrasChave: [
      'Valores de senha/API key nunca entram no log',
      'Checkout registra motivo, TTL e chaves (não valores)',
    ],
  },
  {
    rota: '/recycle-bin',
    icone: '🗑️',
    titulo: 'Lixeira',
    subtitulo: 'Itens removidos recuperáveis',
    modulo: 'AG-BIN',
    fase: 'Governança',
    corAccent: '#78716c',
    objetivo:
      'Recuperar ou purgar entidades apagadas conforme política do módulo.',
    problema:
      'Delete acidental sem soft-delete gera retrabalho e buraco de audit.',
    comoFunciona: [
      'Lista itens na lixeira',
      'Restaurar ou excluir definitivamente (se permitido)',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Busque o item',
        descricao: 'Confirme data e quem removeu.',
      },
      {
        passo: 2,
        acao: 'Restaure se erro',
        descricao: 'Purge só com certeza.',
      },
    ],
    regrasChave: ['Purge pode ser irreversível'],
  },
  {
    rota: '/settings',
    icone: '🛠️',
    titulo: 'Configurações',
    subtitulo: 'Preferências do console',
    modulo: 'AG-SET',
    fase: 'Core',
    corAccent: '#536471',
    objetivo:
      'Ajustes gerais do Manager (preferências de sessão/UI). Configs do broker de contas estão em Contas da org.',
    problema:
      'Misturar settings de plataforma com settings de produto confunde o admin.',
    comoFunciona: [
      'Preferências do console autenticado',
      'Broker org: webhook/TTL na própria tela Contas da org',
    ],
    fluxoSugerido: [
      {
        passo: 1,
        acao: 'Revise preferências',
        descricao: 'Idioma também está no header.',
      },
      {
        passo: 2,
        acao: 'Broker',
        descricao: 'Webhook dual-control em Contas da org → Configurações.',
      },
    ],
    regrasChave: ['Bootstrap OPENBAO_* continua no deploy, não aqui'],
  },
]

/**
 * Resolve o guia pela rota atual (pathname).
 * Match exato, depois maior prefixo (detalhes /ids herdam o módulo pai).
 */
export function getGuiaByRota(rota: string): GuiaModulo | undefined {
  const path = (rota.split('?')[0] || '/').replace(/\/$/, '') || '/'
  const normalized = path.startsWith('/') ? path : `/${path}`

  const exact = guiaModulos.find((g) => g.rota === normalized)
  if (exact) return exact

  // aliases comuns
  if (normalized === '/' || normalized === '') {
    return guiaModulos.find((g) => g.rota === '/dashboard')
  }

  const sorted = [...guiaModulos].sort((a, b) => b.rota.length - a.rota.length)
  for (const g of sorted) {
    if (g.rota === '/dashboard') continue
    if (normalized === g.rota || normalized.startsWith(`${g.rota}/`)) {
      return g
    }
  }

  return undefined
}

export function getModulosPorFase(): Record<string, GuiaModulo[]> {
  return guiaModulos.reduce(
    (acc, modulo) => {
      const fase = modulo.fase.split(' · ')[0] || modulo.fase
      if (!acc[fase]) acc[fase] = []
      acc[fase].push(modulo)
      return acc
    },
    {} as Record<string, GuiaModulo[]>,
  )
}
