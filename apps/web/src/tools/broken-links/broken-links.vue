<script setup lang="ts">
import type { HttpLinks } from './broken-links.service'
import { NAlert, NButton, NCheckbox, NTag } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { normalizeUrl } from '../social-preview/social-preview.service'
import { filterLinks, statusText } from './broken-links.service'

const { t } = useI18n()
const url = useQueryParam({ name: 'url', defaultValue: '' })
const external = useQueryParam({ name: 'external', defaultValue: true })
const { data, error, loading, run } = useApi<HttpLinks>('/v1/http/links')
const submit = () => url.value.trim() && run({ url: normalizeUrl(url.value), external: String(external.value) })
onMounted(() => {
  if (url.value)
    void submit()
})
const onlyBroken = ref(true)
const kind = ref('')
const kinds = computed(() => ['', 'page', 'asset', 'external'].map(k => ({ value: k, label: k ? t(`tools.broken-links.kinds.${k}`) : t('tools.broken-links.allKinds') })))
const rows = computed(() => (data.value ? filterLinks(data.value.links, onlyBroken.value, kind.value) : []).map(l => ({
  status: statusText(l),
  url: l.url,
  kind: t(`tools.broken-links.kinds.${l.kind}`),
  from: l.refs > 1 ? `${l.from} (+${l.refs - 1})` : l.from,
})))
const columns = computed(() => [
  { key: 'status', title: t('tools.broken-links.status'), width: 160 },
  { key: 'url', title: 'URL', mono: true, width: 380, hrefKey: 'url' },
  { key: 'kind', title: t('tools.broken-links.kind'), width: 110 },
  { key: 'from', title: t('tools.broken-links.from'), mono: true, width: 320 },
])
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="url" :label="t('tools.broken-links.url')" placeholder="https://www.example.com/" monospace test-id="url" />
      <NCheckbox v-model:checked="external" class="mb-3">
        {{ t('tools.broken-links.external') }}
      </NCheckbox>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.broken-links.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!url.trim()" data-testid="run">
        {{ t('tools.broken-links.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <NAlert :type="data.broken ? 'warning' : 'success'" class="mb-4" data-testid="summary">
      {{ t('tools.broken-links.summary', { pages: data.pages.length, checked: data.checked, broken: data.broken }) }}
      <template v-if="data.truncated">
        {{ t('tools.broken-links.truncated') }}
      </template>
    </NAlert>
    <CCard :title="t('common.result')" data-testid="result">
      <div class="mb-3 flex flex-wrap items-center gap-x-6 gap-y-2">
        <NCheckbox v-model:checked="onlyBroken">
          {{ t('tools.broken-links.onlyBroken') }}
        </NCheckbox>
        <div class="min-w-44">
          <CSelect v-model:value="kind" :label="t('tools.broken-links.kind')" :options="kinds" />
        </div>
        <NTag size="small" :bordered="false">
          {{ t('tools.broken-links.shown', { n: rows.length }) }}
        </NTag>
      </div>
      <ResultTable :columns="columns" :rows="rows" filename="broken-links.json" :label="t('common.result')" />
    </CCard>
  </template>
</template>
