// src/lib/utils/error-mapper.ts

const KANIDM_ERROR_MAP: Record<string, string> = {
  duplicate_value: 'Este valor já existe no sistema.',
  no_matching_entries: 'Nenhum registro encontrado.',
  access_denied: 'Sem permissão para esta operação.',
  invalid_attribute: 'Atributo inválido.',
  missing_attribute: 'Atributo obrigatório não fornecido.',
  session_expired: 'Sessão expirada. Faça login novamente.',
  account_locked: 'Conta bloqueada por excesso de tentativas.',
  credential_invalid: 'Credenciais inválidas.',
  schema_violation: 'Os dados não seguem o formato esperado.',
}

export function mapKanidmError(error: unknown): string {
  if (error instanceof Error) {
    const match = error.message.match(/Kanidm API (\d+): (.+)/)
    if (match) {
      const [, status, body] = match
      try {
        const parsed = JSON.parse(body)
        return KANIDM_ERROR_MAP[parsed.error] ?? parsed.error ?? `Erro ${status}`
      } catch {
        return body
      }
    }
    return error.message
  }
  return 'Erro desconhecido'
}
