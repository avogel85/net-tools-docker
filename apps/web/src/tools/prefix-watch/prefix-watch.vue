<script setup lang="ts">
import type { Check, PrefixStatus, RpkiResult, Store } from './prefix-watch.service'
import { NAlert, NButton, NFormItem, NInput, NTag } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiGet } from '@/api/client'
import { buildQuery } from '@/composables/useApi'
import CCard from '@/ui/c-card.vue'
import { downloadText } from '@/utils/download'
import { due, findings, level, load, MAX_WATCHES, parseWatches, PER_RUN, save } from './prefix-watch.service'

const { t, locale } = useI18n()
const store = ref<Store>(load())
const storageOk = ref(true)
function persist() {
  storageOk.value = save(store.value)
}
const input = ref('')
const invalid = ref<string[]>([])
function add() {
  const r = parseWatches(input.value)
  invalid.value = r.invalid
  const merged = [...store.value.watches.filter(w => !r.watches.some(x => x.prefix === w.prefix)), ...r.watches].slice(0, MAX_WATCHES)
  store.value = { ...store.value, watches: merged }
  input.value = r.invalid.join('\n')
  persist()
  void check()
}
function remove(prefix: string) {
  store.value = { watches: store.value.watches.filter(w => w.prefix !== prefix), checks: store.value.checks.filter(c => c.prefix !== prefix) }
  persist()
}
const busy = ref(false)
const skipped = ref(0)
async function check(force = false) {
  if (busy.value)
    return
  busy.value = true
  const list = force ? store.value.watches.slice(0, PER_RUN) : due(store.value.watches, store.value.checks)
  skipped.value = Math.max(0, store.value.watches.length - list.length)
  const fresh: Check[] = []
  // Sequential: the RIPEstat-backed endpoints are rate-limited per visitor.
  for (const w of list) {
    const c: Check = { prefix: w.prefix, checkedAt: new Date().toISOString(), announced: false, origins: [], more: [], rpki: {}, error: '' }
    try {
      const st = await apiGet<PrefixStatus>(`/v1/prefix/status${buildQuery({ prefix: w.prefix })}`)
      c.announced = st.announced
      c.origins = st.origins.map(o => o.asn)
      c.more = st.moreSpecifics.slice(0, 50).map(m => ({ prefix: m.prefix, origin: m.origin }))
      const rp = await apiGet<RpkiResult>(`/v1/rpki${buildQuery({ prefix: w.prefix, asn: String(w.origins[0]) })}`)
      c.rpki[w.origins[0]!] = rp.status
    }
    catch (e) {
      c.error = e instanceof Error ? e.message : String(e)
    }
    fresh.push(c)
  }
  store.value = { ...store.value, checks: [...store.value.checks.filter(c => !fresh.some(f => f.prefix === c.prefix)), ...fresh] }
  persist()
  busy.value = false
}
onMounted(() => {
  void check()
})
const rows = computed(() => store.value.watches.map((w) => {
  const c = store.value.checks.find(x => x.prefix === w.prefix)
  const f = c ? findings(w, c) : []
  return { w, c, f, lvl: c ? level(f) : undefined }
}).sort((a, b) => ['alert', 'warn', 'ok', undefined].indexOf(a.lvl) - ['alert', 'warn', 'ok', undefined].indexOf(b.lvl)))
const tag = { alert: 'error', warn: 'warning', ok: 'success' } as const
const when = (iso: string) => new Date(iso).toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
</script>

<template>
  <CCard>
    <NFormItem :label="t('tools.prefix-watch.add', { max: MAX_WATCHES })" :validation-status="invalid.length ? 'error' : undefined" :feedback="invalid.length ? t('tools.prefix-watch.invalid', { list: invalid.join(', ') }) : undefined">
      <NInput v-model:value="input" type="textarea" :autosize="{ minRows: 2, maxRows: 8 }" class="font-mono" placeholder="192.0.2.0/24 AS64500&#10;2001:db8::/32 AS64500" :input-props="{ 'aria-label': t('tools.prefix-watch.add', { max: MAX_WATCHES }), 'spellcheck': false }" data-testid="input" />
    </NFormItem>
    <div class="flex flex-wrap gap-3">
      <NButton type="primary" :disabled="!input.trim()" data-testid="add" @click="add">
        {{ t('tools.prefix-watch.addButton') }}
      </NButton>
      <NButton secondary :loading="busy" :disabled="!store.watches.length" data-testid="run" @click="check(true)">
        {{ t('tools.prefix-watch.check') }}
      </NButton>
    </div>
  </CCard>
  <NAlert v-if="!storageOk" type="warning" class="mb-4">
    {{ t('tools.prefix-watch.storageFailed') }}
  </NAlert>
  <NAlert v-if="skipped" type="info" class="mb-4">
    {{ t('tools.prefix-watch.skipped', { n: skipped, per: PER_RUN }) }}
  </NAlert>
  <CCard v-if="rows.length" :title="t('tools.prefix-watch.watched', { n: rows.length })" data-testid="result">
    <div v-for="r in rows" :key="r.w.prefix" class="mb-3 border-b nt-border border-b-solid pb-3 last:border-b-0">
      <div class="flex flex-wrap items-center gap-2">
        <NTag size="small" :type="r.lvl ? tag[r.lvl] : 'default'" :bordered="false">
          {{ r.lvl ? t(`tools.prefix-watch.levels.${r.lvl}`) : t('tools.prefix-watch.notChecked') }}
        </NTag>
        <span class="font-semibold font-mono">{{ r.w.prefix }}</span>
        <span class="text-sm opacity-75">{{ t('tools.prefix-watch.expected', { asns: r.w.origins.map(a => `AS${a}`).join(', ') }) }}</span>
        <NButton size="tiny" secondary class="ml-auto" @click="remove(r.w.prefix)">
          {{ t('tools.prefix-watch.remove') }}
        </NButton>
      </div>
      <div v-if="r.c" class="mt-1 text-sm">
        {{ t('tools.prefix-watch.seen', { origins: r.c.origins.map(a => `AS${a}`).join(', ') || '—', rpki: Object.values(r.c.rpki).join(', ') || '—', when: when(r.c.checkedAt) }) }}
      </div>
      <ul v-if="r.f.length" class="m-0 mt-1 pl-5 text-sm">
        <li v-for="f in r.f" :key="f">
          {{ t(`tools.prefix-watch.findings.${f}`, { list: r.c?.more.map(m => `${m.prefix} AS${m.origin}`).slice(0, 5).join(', ') }) }}
        </li>
      </ul>
    </div>
    <div class="mt-2 flex flex-wrap gap-2">
      <NButton size="small" secondary @click="downloadText('prefix-watch.json', JSON.stringify({ watches: store.watches }, null, 2))">
        {{ t('tools.prefix-watch.export') }}
      </NButton>
    </div>
    <p class="mb-0 mt-3 text-sm">
      {{ t('tools.prefix-watch.hint') }}
    </p>
  </CCard>
</template>
