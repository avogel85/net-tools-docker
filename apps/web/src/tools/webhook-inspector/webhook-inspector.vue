<script setup lang="ts">
import type { Captured, WebhookBin, WebhookContent } from './webhook-inspector.service'
import { NAlert, NButton, NCollapse, NCollapseItem, NTag } from 'naive-ui'
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiDelete, ApiError as ApiErr, apiGet, apiPost } from '@/api/client'
import ApiError from '@/components/ApiError.vue'
import CopyButton from '@/components/CopyButton.vue'
import CCard from '@/ui/c-card.vue'
import { curlExample, loadBin, prettyBody, saveBin } from './webhook-inspector.service'

const { t } = useI18n()
const bin = shallowRef<WebhookBin | undefined>(loadBin())
const content = shallowRef<WebhookContent>()
const error = shallowRef<ApiErr>()
const creating = ref(false)
const hookUrl = computed(() => (bin.value ? `${globalThis.location.origin}${bin.value.path}` : ''))
const now = ref(Date.now())
const minutesLeft = computed(() => (bin.value ? Math.max(0, Math.ceil((Date.parse(bin.value.expires) - now.value) / 60000)) : 0))

async function create() {
  creating.value = true
  error.value = undefined
  try {
    bin.value = await apiPost<WebhookBin>('/v1/webhook/bins', {})
    saveBin(bin.value)
    content.value = undefined
    void poll()
  }
  catch (e) {
    error.value = e instanceof ApiErr ? e : new ApiErr('NETWORK_ERROR', String(e))
  }
  finally {
    creating.value = false
  }
}
async function poll() {
  if (!bin.value)
    return
  try {
    content.value = await apiGet<WebhookContent>(`/v1/webhook/bins/${bin.value.id}`, { headers: { 'X-Bin-Token': bin.value.token } })
  }
  catch (e) {
    if (e instanceof ApiErr && e.code === 'NOT_FOUND') {
      bin.value = undefined
      saveBin(undefined)
    }
  }
}
async function remove() {
  if (!bin.value)
    return
  await apiDelete(`/v1/webhook/bins/${bin.value.id}`, { headers: { 'X-Bin-Token': bin.value.token } }).catch(() => {})
  bin.value = undefined
  content.value = undefined
  saveBin(undefined)
}
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  void poll()
  timer = setInterval(() => {
    now.value = Date.now()
    if (bin.value && document.visibilityState === 'visible')
      void poll()
  }, 3000)
})
onBeforeUnmount(() => clearInterval(timer))
const requests = computed(() => [...(content.value?.requests ?? [])].reverse())
const label = (r: Captured) => `${new Date(r.time).toLocaleTimeString()} · ${r.method} ${r.path}${r.query ? `?${r.query}` : ''}`
</script>

<template>
  <CCard>
    <p class="mt-0 text-sm">
      {{ t('tools.webhook-inspector.intro') }}
    </p>
    <div v-if="!bin" class="flex flex-wrap gap-3">
      <NButton type="primary" :loading="creating" data-testid="create" @click="create">
        {{ t('tools.webhook-inspector.create') }}
      </NButton>
    </div>
    <template v-else>
      <div class="mb-2 text-sm font-semibold">
        {{ t('tools.webhook-inspector.yourUrl') }}
      </div>
      <div class="mb-2 flex flex-wrap items-center gap-2">
        <code class="break-all text-sm font-mono" data-testid="hook-url">{{ hookUrl }}</code>
        <CopyButton :text="hookUrl" size="tiny" />
      </div>
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <code class="break-all text-xs font-mono opacity-80">{{ curlExample(hookUrl) }}</code>
        <CopyButton :text="curlExample(hookUrl)" size="tiny" :label="t('tools.webhook-inspector.copyCurl')" />
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <NTag size="small" :bordered="false">
          {{ t('tools.webhook-inspector.expires', { n: minutesLeft }) }}
        </NTag>
        <NButton size="small" secondary data-testid="delete" @click="remove">
          {{ t('tools.webhook-inspector.delete') }}
        </NButton>
      </div>
    </template>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <NAlert type="info" class="mb-4">
    {{ t('tools.webhook-inspector.privacy') }}
  </NAlert>
  <CCard v-if="bin" :title="t('tools.webhook-inspector.received', { n: content?.requests.length ?? 0 })" data-testid="result">
    <p v-if="!requests.length" class="m-0 text-sm" data-testid="waiting">
      {{ t('tools.webhook-inspector.waiting') }}
    </p>
    <NAlert v-if="content?.dropped" type="warning" class="mb-3">
      {{ t('tools.webhook-inspector.dropped', { n: content.dropped }) }}
    </NAlert>
    <NCollapse v-if="requests.length" :default-expanded-names="[0]">
      <NCollapseItem v-for="(r, i) in requests" :key="r.time + i" :name="i" :title="label(r)" data-testid="request">
        <div class="mb-2 text-sm">
          {{ t('tools.webhook-inspector.from', { ip: r.remoteIp }) }} · {{ r.contentType || '—' }} · {{ r.bodyBytes }} B<span v-if="r.truncated"> ({{ t('tools.webhook-inspector.truncated') }})</span>
        </div>
        <dl class="grid grid-cols-1 m-0 mb-3 gap-x-4 text-xs md:grid-cols-[14rem_1fr]">
          <template v-for="(h, j) in r.headers" :key="j">
            <dt class="font-semibold font-mono">
              {{ h.name }}
            </dt>
            <dd class="m-0 break-all font-mono">
              {{ h.value }}
            </dd>
          </template>
        </dl>
        <p v-if="r.binary" class="m-0 text-sm">
          {{ t('tools.webhook-inspector.binary') }}
        </p>
        <template v-else-if="r.body">
          <pre class="m-0 mb-2 max-h-96 overflow-auto whitespace-pre-wrap break-all text-xs font-mono">{{ prettyBody(r) }}</pre>
          <CopyButton :text="r.body" size="tiny" />
        </template>
      </NCollapseItem>
    </NCollapse>
  </CCard>
</template>
