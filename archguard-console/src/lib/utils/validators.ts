// src/lib/utils/validators.ts
import { z } from 'zod'

// ── Person ──────────────────────────────────
export const createPersonSchema = z.object({
  name: z
    .string()
    .min(2, 'Mínimo 2 caracteres')
    .max(64, 'Máximo 64 caracteres')
    .regex(/^[a-z][a-z0-9._-]+$/, 'Apenas letras minúsculas, números, . _ -'),
  displayname: z.string().min(1, 'Obrigatório').max(128),
  legalname: z.string().max(128).optional(),
  mail: z.array(z.string().email('Email inválido')).min(1, 'Pelo menos um email'),
  groups: z.array(z.string()).optional(),
})
export type CreatePersonInput = z.infer<typeof createPersonSchema>

// ── Group ───────────────────────────────────
export const createGroupSchema = z.object({
  name: z
    .string()
    .min(2)
    .max(64)
    .regex(/^[a-z][a-z0-9_-]+$/, 'Apenas letras minúsculas, números, _ -'),
  description: z.string().max(256).optional(),
  members: z.array(z.string()).optional(),
})
export type CreateGroupInput = z.infer<typeof createGroupSchema>

// ── OAuth2 Client ───────────────────────────
export const createOAuth2Schema = z.object({
  name: z
    .string()
    .min(2)
    .max(64)
    .regex(/^[a-z][a-z0-9-]+$/, 'Apenas letras minúsculas, números e hífens'),
  displayname: z.string().min(1).max(128),
  origin_landing: z.string().url('URL inválida'),
  type: z.enum(['basic', 'public']),
  redirect_urls: z.array(z.string().url()).min(1, 'Pelo menos uma redirect URL'),
})
export type CreateOAuth2Input = z.infer<typeof createOAuth2Schema>

// ── Service Account ─────────────────────────
export const createServiceAccountSchema = z.object({
  name: z
    .string()
    .min(2)
    .max(64)
    .regex(/^[a-z][a-z0-9_-]+$/),
  displayname: z.string().min(1).max(128),
  description: z.string().max(256).optional(),
  groups: z.array(z.string()).optional(),
})
export type CreateServiceAccountInput = z.infer<typeof createServiceAccountSchema>

// ── Search Params (URL state) ───────────────
export const searchParamsSchema = z.object({
  q: z.string().optional(),
  page: z.number().int().min(1).default(1),
  pageSize: z.number().int().min(10).max(100).default(25),
  sortBy: z.string().optional(),
  sortDir: z.enum(['asc', 'desc']).default('asc'),
  status: z.enum(['active', 'expired', 'locked', 'all']).default('all'),
  group: z.string().optional(),
})
export type SearchParams = z.infer<typeof searchParamsSchema>

// ── Audit Filters ───────────────────────────
export const auditFiltersSchema = z.object({
  period: z.enum(['1h', '24h', '7d', '30d', 'custom']).default('24h'),
  dateFrom: z.string().datetime().optional(),
  dateTo: z.string().datetime().optional(),
  eventType: z.string().optional(),
  actor: z.string().optional(),
  target: z.string().optional(),
  status: z
    .array(z.enum(['success', 'failure', 'alert']))
    .default(['success', 'failure', 'alert']),
})
export type AuditFilters = z.infer<typeof auditFiltersSchema>
