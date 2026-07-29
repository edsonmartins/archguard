// JSON-shaped types for payloads that cross the server-function boundary.
//
// `Record<string, unknown>` does not satisfy TanStack's serializable
// constraint (`unknown` admits undefined, which cannot be serialized), so
// handlers echoing opaque upstream JSON were failing typecheck. These types
// describe what actually travels: parsed JSON.

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue }

export type JsonObject = { [key: string]: JsonValue }
