// src/components/identity/csv-import-wizard.tsx

import { useState, useRef } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  ArrowLeft,
  ArrowRight,
  Upload,
  Columns,
  CheckCircle2,
  Loader2,
  AlertCircle,
  Check,
  X,
  FileSpreadsheet,
} from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ScrollArea } from '@/components/ui/scroll-area'
import { personApi } from '@/lib/api/kanidm-client'
import {
  parseCsv,
  mapCsvRows,
  validateCsvPersons,
  MAX_CSV_IMPORT,
  type CsvParseResult,
  type ColumnMapping,
  type CsvPersonRow,
  type ValidationResult,
} from '@/lib/utils/csv-parser'

const STEPS = [
  { id: 'upload', label: 'Upload', icon: Upload },
  { id: 'mapping', label: 'Mapeamento', icon: Columns },
  { id: 'validate', label: 'Validação', icon: CheckCircle2 },
  { id: 'progress', label: 'Importação', icon: Loader2 },
] as const

interface ImportStatus {
  total: number
  completed: number
  succeeded: number
  failed: number
  results: { username: string; success: boolean; error?: string }[]
}

export function CsvImportWizard() {
  const navigate = useNavigate()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [step, setStep] = useState(0)

  // Step 1: Upload
  const [csvData, setCsvData] = useState<CsvParseResult | null>(null)
  const [fileName, setFileName] = useState('')

  // Step 2: Mapping
  const [mapping, setMapping] = useState<ColumnMapping>({
    username: 0,
    displayname: 1,
    email: 2,
  })

  // Step 3: Validation
  const [validation, setValidation] = useState<ValidationResult | null>(null)

  // Step 4: Import
  const [importStatus, setImportStatus] = useState<ImportStatus | null>(null)
  const [isImporting, setIsImporting] = useState(false)

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    setFileName(file.name)
    const reader = new FileReader()
    reader.onload = (ev) => {
      const text = ev.target?.result as string
      const parsed = parseCsv(text)

      if (parsed.rowCount > MAX_CSV_IMPORT) {
        alert(`Máximo de ${MAX_CSV_IMPORT} registros por importação. O arquivo contém ${parsed.rowCount}.`)
        return
      }

      setCsvData(parsed)

      // Auto-detect mapping based on headers
      const headers = parsed.headers.map((h) => h.toLowerCase())
      const autoMapping: ColumnMapping = {
        username: Math.max(0, headers.findIndex((h) => h.includes('user') || h.includes('login') || h === 'name')),
        displayname: Math.max(0, headers.findIndex((h) => h.includes('display') || h.includes('nome') || h.includes('full'))),
        email: Math.max(0, headers.findIndex((h) => h.includes('email') || h.includes('mail'))),
      }

      const legalIdx = headers.findIndex((h) => h.includes('legal') || h.includes('completo'))
      if (legalIdx >= 0) autoMapping.legalname = legalIdx

      const groupIdx = headers.findIndex((h) => h.includes('group') || h.includes('grupo'))
      if (groupIdx >= 0) autoMapping.groups = groupIdx

      setMapping(autoMapping)
    }
    reader.readAsText(file)
  }

  const handleValidate = () => {
    if (!csvData) return
    const mapped = mapCsvRows(csvData.rows, mapping)
    const result = validateCsvPersons(mapped)
    setValidation(result)
    setStep(2)
  }

  const handleImport = async () => {
    if (!validation) return
    setIsImporting(true)

    const status: ImportStatus = {
      total: validation.valid.length,
      completed: 0,
      succeeded: 0,
      failed: 0,
      results: [],
    }
    setImportStatus({ ...status })
    setStep(3)

    // Process in batches with concurrency limit of 5
    const CONCURRENCY = 5
    const queue = [...validation.valid]

    const processOne = async (person: CsvPersonRow) => {
      try {
        await personApi.create({
          name: person.username,
          displayname: person.displayname,
          mail: [person.email],
          legalname: person.legalname,
          groups: person.groups,
        })
        status.succeeded++
        status.results.push({ username: person.username, success: true })
      } catch (err) {
        status.failed++
        status.results.push({
          username: person.username,
          success: false,
          error: err instanceof Error ? err.message : 'Erro desconhecido',
        })
      }
      status.completed++
      setImportStatus({ ...status })
    }

    // Execute with concurrency limit
    const executing = new Set<Promise<void>>()
    for (const person of queue) {
      const p = processOne(person).then(() => {
        executing.delete(p)
      })
      executing.add(p)
      if (executing.size >= CONCURRENCY) {
        await Promise.race(executing)
      }
    }
    await Promise.all(executing)
    setIsImporting(false)
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/identities">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Importar CSV</h1>
          <p className="text-muted-foreground">
            Importe pessoas em lote a partir de um arquivo CSV
          </p>
        </div>
      </div>

      {/* Step indicator */}
      <div className="flex items-center gap-2">
        {STEPS.map((s, i) => {
          const Icon = s.icon
          const isActive = i === step
          const isDone = i < step
          return (
            <div key={s.id} className="flex items-center gap-2">
              {i > 0 && <Separator className="w-6" />}
              <div
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm ${
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : isDone
                      ? 'bg-primary/10 text-primary'
                      : 'text-muted-foreground'
                }`}
              >
                {isDone ? <Check className="h-4 w-4" /> : <Icon className="h-4 w-4" />}
                {s.label}
              </div>
            </div>
          )
        })}
      </div>

      <Card>
        <CardContent className="pt-6">
          {/* Step 0: Upload */}
          {step === 0 && (
            <div className="space-y-4">
              <div
                className="flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed p-12 hover:border-primary/50"
                onClick={() => fileInputRef.current?.click()}
              >
                <FileSpreadsheet className="mb-4 h-12 w-12 text-muted-foreground/50" />
                {csvData ? (
                  <div className="text-center">
                    <p className="font-medium">{fileName}</p>
                    <p className="text-sm text-muted-foreground">
                      {csvData.rowCount} registros encontrados · {csvData.headers.length} colunas
                    </p>
                  </div>
                ) : (
                  <div className="text-center">
                    <p className="font-medium">
                      Clique para selecionar um arquivo CSV
                    </p>
                    <p className="text-sm text-muted-foreground">
                      Máximo {MAX_CSV_IMPORT} registros. Separador: vírgula ou ponto-e-vírgula.
                    </p>
                  </div>
                )}
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".csv,.txt"
                  onChange={handleFileSelect}
                  className="hidden"
                />
              </div>

              {csvData && csvData.rows.length > 0 && (
                <div>
                  <Label className="mb-2 block text-sm font-medium">
                    Pré-visualização (primeiras 5 linhas)
                  </Label>
                  <ScrollArea className="h-48 rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          {csvData.headers.map((h, i) => (
                            <TableHead key={i} className="text-xs">
                              {h}
                            </TableHead>
                          ))}
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {csvData.rows.slice(0, 5).map((row, i) => (
                          <TableRow key={i}>
                            {row.map((cell, j) => (
                              <TableCell key={j} className="text-xs">
                                {cell}
                              </TableCell>
                            ))}
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </ScrollArea>
                </div>
              )}
            </div>
          )}

          {/* Step 1: Column Mapping */}
          {step === 1 && csvData && (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Mapeie as colunas do CSV para os campos do sistema
              </p>
              <div className="grid gap-4 md:grid-cols-2">
                <MappingSelect
                  label="Username *"
                  headers={csvData.headers}
                  value={mapping.username}
                  onChange={(v) => setMapping({ ...mapping, username: v })}
                />
                <MappingSelect
                  label="Nome de Exibição *"
                  headers={csvData.headers}
                  value={mapping.displayname}
                  onChange={(v) => setMapping({ ...mapping, displayname: v })}
                />
                <MappingSelect
                  label="Email *"
                  headers={csvData.headers}
                  value={mapping.email}
                  onChange={(v) => setMapping({ ...mapping, email: v })}
                />
                <MappingSelect
                  label="Nome Legal"
                  headers={csvData.headers}
                  value={mapping.legalname}
                  onChange={(v) =>
                    setMapping({ ...mapping, legalname: v === -1 ? undefined : v })
                  }
                  optional
                />
                <MappingSelect
                  label="Grupos"
                  headers={csvData.headers}
                  value={mapping.groups}
                  onChange={(v) =>
                    setMapping({ ...mapping, groups: v === -1 ? undefined : v })
                  }
                  optional
                />
              </div>
            </div>
          )}

          {/* Step 2: Validation */}
          {step === 2 && validation && (
            <div className="space-y-4">
              <div className="flex gap-4">
                <Card className="flex-1">
                  <CardContent className="pt-4 text-center">
                    <CheckCircle2 className="mx-auto mb-1 h-6 w-6 text-green-500" />
                    <p className="text-2xl font-bold">{validation.valid.length}</p>
                    <p className="text-xs text-muted-foreground">Válidos</p>
                  </CardContent>
                </Card>
                <Card className="flex-1">
                  <CardContent className="pt-4 text-center">
                    <AlertCircle className="mx-auto mb-1 h-6 w-6 text-destructive" />
                    <p className="text-2xl font-bold">{validation.errors.length}</p>
                    <p className="text-xs text-muted-foreground">Erros</p>
                  </CardContent>
                </Card>
              </div>

              {validation.errors.length > 0 && (
                <div>
                  <Label className="mb-2 block text-sm font-medium">
                    Erros de Validação
                  </Label>
                  <ScrollArea className="h-48 rounded-md border">
                    <div className="p-4 space-y-2">
                      {validation.errors.slice(0, 50).map((err, i) => (
                        <div
                          key={i}
                          className="flex items-start gap-2 text-sm"
                        >
                          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
                          <span>
                            <span className="font-mono text-xs text-muted-foreground">
                              Linha {err.row}
                            </span>{' '}
                            {err.message}
                          </span>
                        </div>
                      ))}
                      {validation.errors.length > 50 && (
                        <p className="text-xs text-muted-foreground">
                          ... e mais {validation.errors.length - 50} erros
                        </p>
                      )}
                    </div>
                  </ScrollArea>
                </div>
              )}

              {validation.valid.length > 0 && (
                <div>
                  <Label className="mb-2 block text-sm font-medium">
                    Registros Válidos (primeiros 10)
                  </Label>
                  <ScrollArea className="h-48 rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="text-xs">Username</TableHead>
                          <TableHead className="text-xs">Nome</TableHead>
                          <TableHead className="text-xs">Email</TableHead>
                          <TableHead className="text-xs">Grupos</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {validation.valid.slice(0, 10).map((p) => (
                          <TableRow key={p.username}>
                            <TableCell className="text-xs font-mono">
                              {p.username}
                            </TableCell>
                            <TableCell className="text-xs">
                              {p.displayname}
                            </TableCell>
                            <TableCell className="text-xs">{p.email}</TableCell>
                            <TableCell className="text-xs">
                              {p.groups?.join(', ') ?? '—'}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </ScrollArea>
                </div>
              )}
            </div>
          )}

          {/* Step 3: Import Progress */}
          {step === 3 && importStatus && (
            <div className="space-y-4">
              <div className="text-center">
                {isImporting ? (
                  <Loader2 className="mx-auto mb-2 h-8 w-8 animate-spin text-primary" />
                ) : (
                  <CheckCircle2 className="mx-auto mb-2 h-8 w-8 text-green-500" />
                )}
                <p className="text-lg font-medium">
                  {isImporting
                    ? `Importando... ${importStatus.completed}/${importStatus.total}`
                    : 'Importação concluída'}
                </p>
              </div>

              <Progress
                value={(importStatus.completed / importStatus.total) * 100}
              />

              <div className="flex gap-4">
                <Card className="flex-1">
                  <CardContent className="pt-4 text-center">
                    <p className="text-2xl font-bold text-green-600">
                      {importStatus.succeeded}
                    </p>
                    <p className="text-xs text-muted-foreground">Criados</p>
                  </CardContent>
                </Card>
                <Card className="flex-1">
                  <CardContent className="pt-4 text-center">
                    <p className="text-2xl font-bold text-destructive">
                      {importStatus.failed}
                    </p>
                    <p className="text-xs text-muted-foreground">Erros</p>
                  </CardContent>
                </Card>
              </div>

              {importStatus.results.length > 0 && (
                <ScrollArea className="h-48 rounded-md border">
                  <div className="p-4 space-y-1">
                    {importStatus.results.map((r) => (
                      <div
                        key={r.username}
                        className="flex items-center gap-2 text-sm"
                      >
                        {r.success ? (
                          <Check className="h-4 w-4 text-green-500" />
                        ) : (
                          <X className="h-4 w-4 text-destructive" />
                        )}
                        <span className="font-mono text-xs">
                          @{r.username}
                        </span>
                        {r.error && (
                          <span className="text-xs text-destructive">
                            {r.error}
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                </ScrollArea>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button
          variant="outline"
          onClick={() => {
            if (step === 0) navigate({ to: '/identities' })
            else if (!isImporting) setStep(step - 1)
          }}
          disabled={isImporting}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          {step === 0 ? 'Cancelar' : 'Voltar'}
        </Button>

        {step === 0 && (
          <Button onClick={() => setStep(1)} disabled={!csvData}>
            Próximo
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        )}

        {step === 1 && (
          <Button onClick={handleValidate}>
            Validar
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        )}

        {step === 2 && validation && validation.valid.length > 0 && (
          <Button onClick={handleImport}>
            Importar {validation.valid.length} registros
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        )}

        {step === 3 && !isImporting && (
          <Button onClick={() => navigate({ to: '/identities' })}>
            Concluir
            <Check className="ml-2 h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  )
}

function MappingSelect({
  label,
  headers,
  value,
  onChange,
  optional,
}: {
  label: string
  headers: string[]
  value: number | undefined
  onChange: (value: number) => void
  optional?: boolean
}) {
  return (
    <div className="space-y-2">
      <Label className="text-sm">{label}</Label>
      <Select
        value={value !== undefined ? String(value) : '-1'}
        onValueChange={(v) => onChange(Number(v))}
      >
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {optional && <SelectItem value="-1">— Não mapear —</SelectItem>}
          {headers.map((h, i) => (
            <SelectItem key={i} value={String(i)}>
              {h}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
