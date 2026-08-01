// Group-name normalization across IdPs.
//
// The same logical group arrives in a token under different shapes:
//
//   Kanidm     archguard_users@id.archgate.com.br   (SPN, @domain suffix)
//   ArchGuard  archgate/archguard_users             (Casdoor <owner>/<name>)
//
// Casdoor composes the `owner/` prefix itself when a membership is stored
// (web/src/UserEditPage.js), so the groups are created with plain names and the
// prefix only ever appears in the claim. Both shapes are stripped here so the
// rest of the console compares bare names.
//
// Kept deliberately tolerant: during the migration a session may carry groups
// from either IdP.

/** Strips the Casdoor `<owner>/` prefix and the Kanidm `@domain` suffix. */
export function normalizeGroupName(raw: string): string {
  const withoutDomain = String(raw).replace(/@.*$/, '')
  const lastSegment = withoutDomain.slice(withoutDomain.lastIndexOf('/') + 1)
  return lastSegment.trim()
}

/** Normalizes a claim array, dropping empties and Kanidm's UUID entries. */
export function normalizeGroupNames(raw: readonly string[] | undefined): string[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map(normalizeGroupName)
    .filter((g) => g.length > 0)
    .filter((g) => !/^[0-9a-f]{8}-[0-9a-f]{4}-/.test(g))
}

/**
 * Tenant matching also has to survive the `-` vs `_` split between Kanidm
 * group names (`tenant_rio_quality`) and Warpgate role names
 * (`tenant-rio-quality`).
 */
export function canonicalTenantKey(raw: string): string {
  return normalizeGroupName(raw).replace(/-/g, '_')
}
