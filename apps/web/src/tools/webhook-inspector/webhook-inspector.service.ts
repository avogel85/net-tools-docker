import type { components } from '@/api/schema'

export type WebhookBin = components['schemas']['WebhookBin']
export type WebhookContent = components['schemas']['WebhookContent']
export type Captured = WebhookContent['requests'][number]

const KEY = 'nettools.webhook-bin'

/** The current bin survives a reload in this tab (session storage); failures are ignored. */
export function loadBin(storage: Pick<Storage, 'getItem'> | undefined = globalThis.sessionStorage): WebhookBin | undefined {
  try {
    const b = JSON.parse(storage?.getItem(KEY) ?? 'null') as WebhookBin | null
    return b && typeof b.id === 'string' && Date.parse(b.expires) > Date.now() ? b : undefined
  }
  catch {
    return undefined
  }
}

export function saveBin(b: WebhookBin | undefined, storage: Pick<Storage, 'setItem' | 'removeItem'> | undefined = globalThis.sessionStorage): void {
  try {
    if (b)
      storage?.setItem(KEY, JSON.stringify(b))
    else
      storage?.removeItem(KEY)
  }
  catch {}
}

/** Pretty-prints JSON bodies; other bodies are returned as they are. */
export function prettyBody(c: Pick<Captured, 'body' | 'contentType'>): string {
  if (/json/i.test(c.contentType) || /^\s*[[{]/.test(c.body)) {
    try {
      return JSON.stringify(JSON.parse(c.body), null, 2)
    }
    catch {}
  }
  if (/x-www-form-urlencoded/i.test(c.contentType))
    return [...new URLSearchParams(c.body)].map(([k, v]) => `${k} = ${v}`).join('\n')
  return c.body
}

export function curlExample(url: string): string {
  return `curl -X POST '${url}' -H 'Content-Type: application/json' -d '{"hello":"world"}'`
}
