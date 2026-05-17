// src/components/identity/credential-reset-dialog.tsx

import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CopyButton } from '@/components/shared/copy-button'
import { useResetPersonCredential } from '@/lib/hooks/use-persons'

interface CredentialResetDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  personId: string
  personName: string
}

const TTL_OPTIONS = [
  { label: '1 hora', value: 3600 },
  { label: '4 horas', value: 14400 },
  { label: '24 horas', value: 86400 },
  { label: '7 dias', value: 604800 },
]

export function CredentialResetDialog({
  open,
  onOpenChange,
  personId,
  personName,
}: CredentialResetDialogProps) {
  const resetCredential = useResetPersonCredential()
  const [ttl, setTtl] = useState(3600)
  const [resetToken, setResetToken] = useState<string | null>(null)

  const handleReset = () => {
    resetCredential.mutate(
      { id: personId, ttl },
      {
        onSuccess: (data) => {
          setResetToken(data.token)
        },
      },
    )
  }

  const handleClose = () => {
    setResetToken(null)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reset de Credencial</DialogTitle>
          <DialogDescription>
            Gerar link de reset para {personName}
          </DialogDescription>
        </DialogHeader>

        {!resetToken ? (
          <>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Validade do link</Label>
                <Select
                  value={String(ttl)}
                  onValueChange={(v) => setTtl(Number(v))}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {TTL_OPTIONS.map((opt) => (
                      <SelectItem key={opt.value} value={String(opt.value)}>
                        {opt.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={handleClose}>
                Cancelar
              </Button>
              <Button
                onClick={handleReset}
                disabled={resetCredential.isPending}
              >
                {resetCredential.isPending ? 'Gerando...' : 'Gerar Link'}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <div className="space-y-4 py-4">
              <div className="rounded-lg border bg-muted p-4">
                <Label className="mb-2 block text-xs text-muted-foreground">
                  Link de Reset (copie agora — não será exibido novamente)
                </Label>
                <div className="flex items-center gap-2">
                  <Input
                    readOnly
                    value={resetToken}
                    className="font-mono text-xs"
                  />
                  <CopyButton value={resetToken} />
                </div>
              </div>
              <p className="text-sm text-muted-foreground">
                Envie este link para o usuário de forma segura. O link expira em{' '}
                {TTL_OPTIONS.find((o) => o.value === ttl)?.label ?? `${ttl}s`}.
              </p>
            </div>
            <DialogFooter>
              <Button onClick={handleClose}>Fechar</Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
