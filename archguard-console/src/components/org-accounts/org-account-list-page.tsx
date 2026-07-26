import { useMemo, useState } from 'react'
import {
  Copy,
  ExternalLink,
  KeyRound,
  Lock,
  Plus,
  Search,
  Trash2,
  Unlock,
} from 'lucide-react'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import type {
  OrgAccount,
  OrgAccountAuthKind,
  OrgAccountCategory,
  OrgAccountCriticality,
  OrgAccountInput,
} from '@/lib/api/types/org-account'
import {
  ORG_AUTH_KINDS,
  ORG_CATEGORIES,
  ORG_CRITICALITIES,
} from '@/lib/api/types/org-account'
import {
  useCheckinOrgAccount,
  useCheckoutOrgAccount,
  useDeleteOrgAccount,
  useOrgAccounts,
  useStoreOrgAccountSecret,
  useUpsertOrgAccount,
} from '@/lib/hooks/use-org-accounts'
import type { CheckoutResult } from '@/server/org-accounts-fn'
import { usePermissions } from '@/lib/hooks/use-permissions'
import { PermissionGate } from '@/components/shared/permission-gate'
import { EmptyState } from '@/components/shared/empty-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function blank(): OrgAccountInput {
  return {
    slug: '',
    name: '',
    category: 'other',
    criticality: 'P2',
    auth_kind: 'password',
    product: '',
    url: '',
    login_hint: '',
    secret_ref: '',
    owner_group: '',
    notes: '',
    runbook_url: '',
  }
}

function critVariant(
  c: OrgAccountCriticality,
): 'destructive' | 'default' | 'secondary' {
  if (c === 'P0') return 'destructive'
  if (c === 'P1') return 'default'
  return 'secondary'
}

export function OrgAccountListPage() {
  const { t } = useTranslation()
  const { can } = usePermissions()
  const canAdmin = can('org_accounts:admin')
  const { data, isLoading, isError, error } = useOrgAccounts()
  const upsert = useUpsertOrgAccount()
  const del = useDeleteOrgAccount()
  const checkout = useCheckoutOrgAccount()
  const checkin = useCheckinOrgAccount()
  const storeSecret = useStoreOrgAccountSecret()
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<OrgAccountInput>(blank())
  const [editing, setEditing] = useState<OrgAccount | null>(null)
  const [checkoutAcc, setCheckoutAcc] = useState<OrgAccount | null>(null)
  const [reason, setReason] = useState('')
  const [ttlMin, setTtlMin] = useState(60)
  const [reveal, setReveal] = useState<CheckoutResult | null>(null)
  const [secretAcc, setSecretAcc] = useState<OrgAccount | null>(null)
  const [secretPw, setSecretPw] = useState('')
  const [secretUser, setSecretUser] = useState('')
  const [secretKey, setSecretKey] = useState('')

  const filtered = useMemo(() => {
    const list = data || []
    const needle = q.trim().toLowerCase()
    if (!needle) return list
    return list.filter(
      (a) =>
        a.name.toLowerCase().includes(needle) ||
        a.slug.toLowerCase().includes(needle) ||
        a.product.toLowerCase().includes(needle) ||
        a.category.toLowerCase().includes(needle) ||
        a.login_hint.toLowerCase().includes(needle),
    )
  }, [data, q])

  function openCreate() {
    setEditing(null)
    setForm(blank())
    setOpen(true)
  }

  function openEdit(a: OrgAccount) {
    setEditing(a)
    setForm({
      slug: a.slug,
      name: a.name,
      category: a.category,
      product: a.product,
      url: a.url,
      login_hint: a.login_hint,
      auth_kind: a.auth_kind,
      secret_ref: a.secret_ref,
      criticality: a.criticality,
      owner_group: a.owner_group,
      requires_dual_control: a.requires_dual_control,
      notes: a.notes,
      runbook_url: a.runbook_url,
    })
    setOpen(true)
  }

  async function save() {
    try {
      await upsert.mutateAsync(form)
      toast.success(t('orgAccounts.saved'))
      setOpen(false)
    } catch (e) {
      toast.error((e as Error).message || t('orgAccounts.saveError'))
    }
  }

  async function remove(a: OrgAccount) {
    if (!confirm(t('orgAccounts.confirmDelete', { name: a.name }))) return
    try {
      await del.mutateAsync(a.id)
      toast.success(t('orgAccounts.deleted'))
    } catch (e) {
      toast.error((e as Error).message || t('orgAccounts.deleteError'))
    }
  }

  async function doCheckout() {
    if (!checkoutAcc) return
    try {
      const res = await checkout.mutateAsync({
        id: checkoutAcc.id,
        reason,
        ttl_seconds: Math.max(5, ttlMin) * 60,
      })
      setReveal(res)
      if (res.secret) toast.success(t('orgAccounts.checkoutOk'))
      else toast.message(res.message || t('orgAccounts.checkoutNoSecret'))
    } catch (e) {
      toast.error((e as Error).message || t('orgAccounts.checkoutError'))
    }
  }

  async function doCheckin() {
    if (!reveal?.checkout?.id) return
    try {
      await checkin.mutateAsync(reveal.checkout.id)
      toast.success(t('orgAccounts.checkinOk'))
      setReveal(null)
      setCheckoutAcc(null)
      setReason('')
    } catch (e) {
      toast.error((e as Error).message)
    }
  }

  async function doStoreSecret() {
    if (!secretAcc) return
    try {
      const r = await storeSecret.mutateAsync({
        id: secretAcc.id,
        password: secretPw || undefined,
        username: secretUser || undefined,
        api_key: secretKey || undefined,
      })
      toast.success(t('orgAccounts.secretStored', { ref: r.secret_ref }))
      setSecretAcc(null)
      setSecretPw('')
      setSecretUser('')
      setSecretKey('')
    } catch (e) {
      toast.error((e as Error).message || t('orgAccounts.secretStoreError'))
    }
  }

  function copyText(label: string, text?: string) {
    if (!text) return
    void navigator.clipboard.writeText(text).then(
      () => toast.success(`${label} ${t('common.copied')}`),
      () => toast.error('clipboard failed'),
    )
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t('orgAccounts.title')}
          </h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            {t('orgAccounts.subtitle')}
          </p>
        </div>
        <PermissionGate require={['org_accounts:admin']}>
          <Button type="button" onClick={openCreate}>
            <Plus className="mr-2 h-4 w-4" />
            {t('orgAccounts.add')}
          </Button>
        </PermissionGate>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-medium">
            {t('orgAccounts.inventory')}
          </CardTitle>
          <div className="relative max-w-md">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              className="pl-9"
              placeholder={t('orgAccounts.search')}
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : isError ? (
            <p className="text-sm text-destructive">
              {(error as Error)?.message || t('orgAccounts.loadError')}
            </p>
          ) : filtered.length === 0 ? (
            <EmptyState
              icon={KeyRound}
              title={t('orgAccounts.emptyTitle')}
              description={t('orgAccounts.emptyDesc')}
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('orgAccounts.colName')}</TableHead>
                  <TableHead>{t('orgAccounts.colCategory')}</TableHead>
                  <TableHead>{t('orgAccounts.colCriticality')}</TableHead>
                  <TableHead>{t('orgAccounts.colAuth')}</TableHead>
                  <TableHead>{t('orgAccounts.colLogin')}</TableHead>
                  <TableHead className="text-right">
                    {t('orgAccounts.colActions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((a) => (
                  <TableRow key={a.id}>
                    <TableCell>
                      <div className="font-medium">{a.name}</div>
                      <div className="text-xs text-muted-foreground">
                        {a.slug}
                        {a.product ? ` · ${a.product}` : ''}
                        {a.requires_dual_control ? (
                          <span className="ml-1 text-amber-600">
                            · dual-control
                          </span>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{a.category}</Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={critVariant(a.criticality)}>
                        {a.criticality}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm">{a.auth_kind}</TableCell>
                    <TableCell className="max-w-[10rem] truncate text-sm text-muted-foreground">
                      {a.login_hint || '—'}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex flex-wrap justify-end gap-1">
                        {a.url ? (
                          <Button variant="ghost" size="icon" asChild>
                            <a
                              href={a.url}
                              target="_blank"
                              rel="noreferrer"
                              title={a.url}
                            >
                              <ExternalLink className="h-4 w-4" />
                            </a>
                          </Button>
                        ) : null}
                        <PermissionGate
                          require={['org_accounts:checkout', 'org_accounts:admin']}
                          any
                        >
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => {
                              setCheckoutAcc(a)
                              setReveal(null)
                              setReason('')
                            }}
                          >
                            <Unlock className="mr-1 h-3.5 w-3.5" />
                            {t('orgAccounts.checkout')}
                          </Button>
                        </PermissionGate>
                        {canAdmin ? (
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                setSecretAcc(a)
                                setSecretPw('')
                                setSecretUser(a.login_hint || '')
                                setSecretKey('')
                              }}
                              title={t('orgAccounts.storeSecret')}
                            >
                              <Lock className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => openEdit(a)}
                            >
                              {t('common.edit')}
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => void remove(a)}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          </>
                        ) : null}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          <p className="mt-4 text-xs text-muted-foreground">
            {t('orgAccounts.noSecretNote')}
          </p>
        </CardContent>
      </Card>

      {/* Checkout dialog */}
      <Dialog
        open={!!checkoutAcc}
        onOpenChange={(v) => {
          if (!v) {
            setCheckoutAcc(null)
            setReveal(null)
            setReason('')
          }
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t('orgAccounts.checkoutTitle', {
                name: checkoutAcc?.name || '',
              })}
            </DialogTitle>
          </DialogHeader>
          {!reveal?.secret ? (
            <div className="grid gap-3 py-2">
              <div className="grid gap-1.5">
                <Label>{t('orgAccounts.reason')}</Label>
                <Textarea
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder={t('orgAccounts.reasonPh')}
                  rows={3}
                />
              </div>
              <div className="grid gap-1.5">
                <Label>{t('orgAccounts.ttlMinutes')}</Label>
                <Input
                  type="number"
                  min={5}
                  max={480}
                  value={ttlMin}
                  onChange={(e) => setTtlMin(Number(e.target.value) || 60)}
                />
              </div>
              {checkoutAcc?.requires_dual_control ? (
                <p className="text-xs text-amber-600">
                  {t('orgAccounts.dualNote')}
                </p>
              ) : null}
            </div>
          ) : (
            <div className="grid gap-3 py-2">
              <p className="text-sm text-muted-foreground">
                {reveal.message} · expira{' '}
                {new Date(reveal.checkout.expires_at).toLocaleString()}
              </p>
              {reveal.secret.username ? (
                <div className="flex items-center gap-2">
                  <code className="flex-1 truncate rounded bg-muted px-2 py-1 text-sm">
                    {reveal.secret.username}
                  </code>
                  <Button
                    size="icon"
                    variant="outline"
                    onClick={() =>
                      copyText('user', reveal.secret?.username)
                    }
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              ) : null}
              {reveal.secret.password ? (
                <div className="flex items-center gap-2">
                  <code className="flex-1 truncate rounded bg-muted px-2 py-1 font-mono text-sm">
                    {reveal.secret.password}
                  </code>
                  <Button
                    size="icon"
                    variant="outline"
                    onClick={() =>
                      copyText('password', reveal.secret?.password)
                    }
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              ) : null}
              {reveal.secret.api_key ? (
                <div className="flex items-center gap-2">
                  <code className="flex-1 truncate rounded bg-muted px-2 py-1 font-mono text-xs">
                    {reveal.secret.api_key}
                  </code>
                  <Button
                    size="icon"
                    variant="outline"
                    onClick={() => copyText('api_key', reveal.secret?.api_key)}
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              ) : null}
              {!reveal.secret.password &&
              !reveal.secret.api_key &&
              reveal.secret.fields ? (
                <pre className="max-h-40 overflow-auto rounded bg-muted p-2 text-xs">
                  {JSON.stringify(reveal.secret.fields, null, 2)}
                </pre>
              ) : null}
            </div>
          )}
          <DialogFooter>
            {!reveal?.secret ? (
              <>
                <Button
                  variant="outline"
                  onClick={() => setCheckoutAcc(null)}
                >
                  {t('common.cancel')}
                </Button>
                <Button
                  onClick={() => void doCheckout()}
                  disabled={checkout.isPending || reason.trim().length < 3}
                >
                  {t('orgAccounts.checkout')}
                </Button>
              </>
            ) : (
              <Button onClick={() => void doCheckin()}>
                {t('orgAccounts.checkin')}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Store secret in OpenBao */}
      <Dialog
        open={!!secretAcc}
        onOpenChange={(v) => {
          if (!v) setSecretAcc(null)
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t('orgAccounts.storeSecretTitle', {
                name: secretAcc?.name || '',
              })}
            </DialogTitle>
          </DialogHeader>
          <p className="text-xs text-muted-foreground">
            {t('orgAccounts.storeSecretHint')}{' '}
            <code className="text-[11px]">
              {secretAcc?.secret_ref ||
                `secret/data/org/${secretAcc?.category}/${secretAcc?.slug}`}
            </code>
          </p>
          <div className="grid gap-3 py-2">
            <div className="grid gap-1.5">
              <Label>username</Label>
              <Input
                value={secretUser}
                onChange={(e) => setSecretUser(e.target.value)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label>password</Label>
              <Input
                type="password"
                value={secretPw}
                onChange={(e) => setSecretPw(e.target.value)}
                autoComplete="new-password"
              />
            </div>
            <div className="grid gap-1.5">
              <Label>api_key</Label>
              <Input
                type="password"
                value={secretKey}
                onChange={(e) => setSecretKey(e.target.value)}
                autoComplete="off"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSecretAcc(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              onClick={() => void doStoreSecret()}
              disabled={
                storeSecret.isPending || (!secretPw && !secretKey)
              }
            >
              {t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[90vh] max-w-lg overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editing ? t('orgAccounts.edit') : t('orgAccounts.add')}
            </DialogTitle>
          </DialogHeader>
          <div className="grid gap-3 py-2">
            <div className="grid gap-1.5">
              <Label>slug</Label>
              <Input
                value={form.slug}
                disabled={!!editing}
                onChange={(e) =>
                  setForm((f) => ({ ...f, slug: e.target.value }))
                }
                placeholder="apple-appstore-integrall"
              />
            </div>
            <div className="grid gap-1.5">
              <Label>{t('orgAccounts.colName')}</Label>
              <Input
                value={form.name}
                onChange={(e) =>
                  setForm((f) => ({ ...f, name: e.target.value }))
                }
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label>{t('orgAccounts.colCategory')}</Label>
                <Select
                  value={form.category}
                  onValueChange={(v) =>
                    setForm((f) => ({
                      ...f,
                      category: v as OrgAccountCategory,
                    }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ORG_CATEGORIES.map((c) => (
                      <SelectItem key={c} value={c}>
                        {c}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-1.5">
                <Label>{t('orgAccounts.colCriticality')}</Label>
                <Select
                  value={form.criticality || 'P2'}
                  onValueChange={(v) =>
                    setForm((f) => ({
                      ...f,
                      criticality: v as OrgAccountCriticality,
                      requires_dual_control:
                        v === 'P0' ? true : f.requires_dual_control,
                    }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ORG_CRITICALITIES.map((c) => (
                      <SelectItem key={c} value={c}>
                        {c}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid gap-1.5">
              <Label>{t('orgAccounts.colAuth')}</Label>
              <Select
                value={form.auth_kind || 'password'}
                onValueChange={(v) =>
                  setForm((f) => ({
                    ...f,
                    auth_kind: v as OrgAccountAuthKind,
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ORG_AUTH_KINDS.map((c) => (
                    <SelectItem key={c} value={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label>URL</Label>
              <Input
                value={form.url || ''}
                onChange={(e) =>
                  setForm((f) => ({ ...f, url: e.target.value }))
                }
              />
            </div>
            <div className="grid gap-1.5">
              <Label>{t('orgAccounts.colLogin')}</Label>
              <Input
                value={form.login_hint || ''}
                onChange={(e) =>
                  setForm((f) => ({ ...f, login_hint: e.target.value }))
                }
                placeholder="email@integrall.tech"
              />
            </div>
            <div className="grid gap-1.5">
              <Label>secret_ref (OpenBao)</Label>
              <Input
                value={form.secret_ref || ''}
                onChange={(e) =>
                  setForm((f) => ({ ...f, secret_ref: e.target.value }))
                }
                placeholder="secret/data/org/store/…"
              />
            </div>
            <div className="grid gap-1.5">
              <Label>product / owner_group</Label>
              <div className="grid grid-cols-2 gap-2">
                <Input
                  value={form.product || ''}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, product: e.target.value }))
                  }
                  placeholder="vendax"
                />
                <Input
                  value={form.owner_group || ''}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, owner_group: e.target.value }))
                  }
                  placeholder="archguard_super_admins"
                />
              </div>
            </div>
            <div className="grid gap-1.5">
              <Label>{t('orgAccounts.notes')}</Label>
              <Textarea
                value={form.notes || ''}
                onChange={(e) =>
                  setForm((f) => ({ ...f, notes: e.target.value }))
                }
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              onClick={() => void save()}
              disabled={upsert.isPending || !form.slug || !form.name}
            >
              {t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
