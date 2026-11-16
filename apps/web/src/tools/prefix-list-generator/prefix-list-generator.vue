<script setup lang="ts">
import type { IrrResult } from '../irr-expansion/irr-expansion.service'
import type { Platform } from './prefix-list-generator.service'
import { NButton, NCheckbox, NTabPane, NTabs, NTag } from 'naive-ui'
import { computed, onMounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError as ApiErr, apiGet } from '@/api/client'
import ApiError from '@/components/ApiError.vue'
import CopyButton from '@/components/CopyButton.vue'
import { buildQuery } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { downloadText } from '@/utils/download'
import { isSetName } from '../irr-expansion/irr-expansion.service'
import { isValidListName, PLATFORMS, render } from './prefix-list-generator.service'

const { t } = useI18n()
const set = useQueryParam({ name: 'set', defaultValue: '' })
const name = useQueryParam({ name: 'name', defaultValue: '' })
const v4 = useQueryParam({ name: 'v4', defaultValue: true })
const v6 = useQueryParam({ name: 'v6', defaultValue: true })
const setV = useValidation({ source: set, rules: [{ validator: isSetName, message: t('tools.irr-expansion.invalid') }] })
const listName = computed(() => name.value.trim() || set.value.trim().toUpperCase().replace(/[^\w-]/g, '-').replace(/^(\d)/, 'AS$1') || 'PEER')
const nameOk = computed(() => isValidListName(listName.value))
const results = shallowRef<{ 4?: IrrResult, 6?: IrrResult }>({})
const error = shallowRef<ApiErr>()
const loading = ref(false)
async function run() {
  if (!set.value.trim() || !setV.value.isValid || (!v4.value && !v6.value))
    return
  loading.value = true
  error.value = undefined
  try {
    const fetchFam = (f: 4 | 6) => apiGet<IrrResult>(`/v1/bgp/irr${buildQuery({ set: set.value.trim(), mode: 'prefixes', family: String(f), aggregate: 'true' })}`)
    const [a, b] = await Promise.all([v4.value ? fetchFam(4) : undefined, v6.value ? fetchFam(6) : undefined])
    results.value = { 4: a, 6: b }
  }
  catch (e) {
    results.value = {}
    error.value = e instanceof ApiErr ? e : new ApiErr('NETWORK_ERROR', String(e))
  }
  finally {
    loading.value = false
  }
}
onMounted(() => {
  if (set.value)
    void run()
})
const tab = ref<Platform>('ios')
const config = (p: Platform) => [results.value[4] && render(results.value[4].prefixes, p, listName.value, 4), results.value[6] && render(results.value[6].prefixes, p, listName.value, 6)].filter(Boolean).join('\n')
const has = computed(() => !!(results.value[4] || results.value[6]))
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="run">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
        <CInputText v-model:value="set" :label="t('tools.prefix-list-generator.set')" placeholder="AS-EXAMPLE, AS65000" monospace test-id="set" :validation="setV" />
        <CInputText v-model:value="name" :label="t('tools.prefix-list-generator.name')" :placeholder="listName" monospace test-id="name" :validation="nameOk ? undefined : { isValid: false, status: 'error', message: t('tools.prefix-list-generator.invalidName') }" />
      </div>
      <div class="mb-3 flex flex-wrap gap-x-6">
        <NCheckbox v-model:checked="v4">
          IPv4
        </NCheckbox>
        <NCheckbox v-model:checked="v6">
          IPv6
        </NCheckbox>
      </div>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!set.trim() || !setV.isValid || !nameOk || (!v4 && !v6)" data-testid="run">
        {{ t('tools.prefix-list-generator.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="has" :title="t('tools.prefix-list-generator.config')" data-testid="result">
    <div class="mb-3 flex flex-wrap gap-2">
      <NTag v-if="results[4]" size="small" :bordered="false">
        IPv4: {{ results[4].prefixes.length }}
      </NTag>
      <NTag v-if="results[6]" size="small" :bordered="false">
        IPv6: {{ results[6].prefixes.length }}
      </NTag>
      <NTag size="small" :bordered="false">
        {{ (results[4] ?? results[6])!.sources.join(', ') }}
      </NTag>
    </div>
    <NTabs v-model:value="tab" type="line" animated>
      <NTabPane v-for="p in PLATFORMS" :key="p" :name="p" :tab="t(`tools.prefix-list-generator.platforms.${p}`)">
        <pre class="m-0 mb-2 max-h-[32rem] overflow-auto text-xs font-mono" :data-testid="`config-${p}`">{{ config(p) }}</pre>
        <div class="flex flex-wrap gap-2">
          <CopyButton :text="config(p)" />
          <NButton secondary size="small" @click="downloadText(`${listName}-${p}.txt`, config(p), 'text/plain')">
            {{ t('tools.prefix-list-generator.download') }}
          </NButton>
        </div>
      </NTabPane>
    </NTabs>
    <p class="mb-0 mt-3 text-sm">
      {{ t('tools.prefix-list-generator.hint') }}
    </p>
  </CCard>
</template>
