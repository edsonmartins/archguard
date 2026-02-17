// src/components/shared/confirm-dialog.tsx

import { useState } from 'react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  confirmText: string
  destructive?: boolean
  onConfirm: () => void
  isLoading?: boolean
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmText,
  destructive,
  onConfirm,
  isLoading,
}: ConfirmDialogProps) {
  const [inputValue, setInputValue] = useState('')

  const isConfirmed = inputValue === confirmText

  const handleConfirm = () => {
    if (isConfirmed) {
      onConfirm()
      setInputValue('')
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <div className="space-y-2 py-2">
          <Label htmlFor="confirm-input">
            Digite <span className="font-mono font-bold">{confirmText}</span>{' '}
            para confirmar
          </Label>
          <Input
            id="confirm-input"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder={confirmText}
          />
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={() => setInputValue('')}>
            Cancelar
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={!isConfirmed || isLoading}
            onClick={handleConfirm}
            className={
              destructive
                ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
                : undefined
            }
          >
            {isLoading ? 'Processando...' : 'Confirmar'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
