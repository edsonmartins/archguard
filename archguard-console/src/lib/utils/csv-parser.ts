// src/lib/utils/csv-parser.ts

export interface CsvParseResult {
  headers: string[]
  rows: string[][]
  rowCount: number
}

export function parseCsv(text: string): CsvParseResult {
  const lines = text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l.length > 0)

  if (lines.length === 0) {
    return { headers: [], rows: [], rowCount: 0 }
  }

  const headers = parseCsvLine(lines[0]!)
  const rows = lines.slice(1).map(parseCsvLine)

  return { headers, rows, rowCount: rows.length }
}

function parseCsvLine(line: string): string[] {
  const result: string[] = []
  let current = ''
  let inQuotes = false

  for (let i = 0; i < line.length; i++) {
    const char = line[i]!

    if (inQuotes) {
      if (char === '"') {
        if (i + 1 < line.length && line[i + 1] === '"') {
          current += '"'
          i++
        } else {
          inQuotes = false
        }
      } else {
        current += char
      }
    } else {
      if (char === '"') {
        inQuotes = true
      } else if (char === ',' || char === ';') {
        result.push(current.trim())
        current = ''
      } else {
        current += char
      }
    }
  }

  result.push(current.trim())
  return result
}

export interface ColumnMapping {
  username: number
  displayname: number
  email: number
  legalname?: number
  groups?: number
}

export interface CsvPersonRow {
  username: string
  displayname: string
  email: string
  legalname?: string
  groups?: string[]
}

export function mapCsvRows(
  rows: string[][],
  mapping: ColumnMapping,
): CsvPersonRow[] {
  return rows
    .map((row) => ({
      username: row[mapping.username]?.trim() ?? '',
      displayname: row[mapping.displayname]?.trim() ?? '',
      email: row[mapping.email]?.trim() ?? '',
      legalname:
        mapping.legalname !== undefined
          ? row[mapping.legalname]?.trim()
          : undefined,
      groups:
        mapping.groups !== undefined && row[mapping.groups]
          ? row[mapping.groups]
              .split(/[;|]/)
              .map((g) => g.trim())
              .filter(Boolean)
          : undefined,
    }))
    .filter((r) => r.username && r.displayname && r.email)
}

export interface ValidationResult {
  valid: CsvPersonRow[]
  errors: { row: number; field: string; message: string }[]
}

export function validateCsvPersons(persons: CsvPersonRow[]): ValidationResult {
  const valid: CsvPersonRow[] = []
  const errors: { row: number; field: string; message: string }[] = []
  const seenUsernames = new Set<string>()

  persons.forEach((person, i) => {
    const rowErrors: { field: string; message: string }[] = []

    if (!/^[a-z][a-z0-9._-]+$/.test(person.username)) {
      rowErrors.push({
        field: 'username',
        message: `Username inválido: "${person.username}"`,
      })
    }

    if (seenUsernames.has(person.username)) {
      rowErrors.push({
        field: 'username',
        message: `Username duplicado: "${person.username}"`,
      })
    }
    seenUsernames.add(person.username)

    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(person.email)) {
      rowErrors.push({
        field: 'email',
        message: `Email inválido: "${person.email}"`,
      })
    }

    if (person.displayname.length < 1) {
      rowErrors.push({
        field: 'displayname',
        message: 'Nome de exibição obrigatório',
      })
    }

    if (rowErrors.length > 0) {
      rowErrors.forEach((e) => errors.push({ row: i + 1, ...e }))
    } else {
      valid.push(person)
    }
  })

  return { valid, errors }
}

export const MAX_CSV_IMPORT = 500
