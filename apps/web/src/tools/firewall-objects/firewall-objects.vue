<script setup lang="ts">
import type { Platform } from './firewall-objects.service'
import { NAlert, NButton, NCheckbox, NFormItem, NInput, NTabPane, NTabs, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { downloadText } from '@/utils/download'
import { isValidName, normalize, parseList, PLATFORMS, render } from './firewall-objects.service'

const { t } = useI18n()
const text = useQueryParam({ name: 'list', defaultValue: '' })
const name = useQueryParam({ name: 'name', defaultValue: 'NETTOOLS_LIST' })
const merge = useQueryParam({ name: 'merge', defaultValue: true })
const parsed = computed(() => parseList(text.value))
const entries = computed(() => normalize(parsed.value.entries, merge.value))
const nameOk = computed(() => isValidName(name.value))
const tab = ref<Platform>('fortigate')
const outputs = computed(() => Object.fromEntries(PLATFORMS.map(p => [p, render(entries.value, p, name.value)])) as Record<Platform, string>)
const counts = computed(() => ({ v4: entries.value.filter(e => e.cidr.version === 4).length, v6: entries.value.filter(e => e.cidr.version === 6).length }))
</script>

<template>
  <CCard>
    <NFormItem :label="t('tools.firewall-objects.list')">
      <NInput v-model:value="text" type="textarea" :autosize="{ minRows: 5, maxRows: 18 }" class="font-mono" placeholder="192.0.2.10 # web server&#10;198.51.100.0/24&#10;203.0.113.1-203.0.113.20&#10;2001:db8::/48" :input-props="{ 'aria-label': t('tools.firewall-objects.list'), 'spellcheck': false }" data-testid="list" />
    </NFormItem>
    <div class="grid grid-cols-1 items-start gap-x-4 md:grid-cols-2">
      <CInputText v-model:value="name" :label="t('tools.firewall-objects.name')" monospace test-id="name" :validation="nameOk ? undefined : { isValid: false, status: 'error', message: t('tools.firewall-objects.invalidName') }" />
      <NCheckbox v-model:checked="merge" class="md:mt-8">
        {{ t('tools.firewall-objects.merge') }}
      </NCheckbox>
    </div>
  </CCard>
  <NAlert v-if="parsed.invalid.length" type="warning" class="mb-4" data-testid="invalid">
    {{ t('tools.firewall-objects.invalid', { list: parsed.invalid.map(i => `${i.line}: ${i.text}`).join(', ') }) }}
  </NAlert>
  <CCard v-if="entries.length && nameOk" :title="t('tools.firewall-objects.output')" data-testid="result">
    <div class="mb-3 flex flex-wrap gap-2">
      <NTag size="small" :bordered="false">
        IPv4: {{ counts.v4 }}
      </NTag>
      <NTag size="small" :bordered="false">
        IPv6: {{ counts.v6 }}
      </NTag>
    </div>
    <NTabs v-model:value="tab" type="line" animated>
      <NTabPane v-for="p in PLATFORMS" :key="p" :name="p" :tab="t(`tools.firewall-objects.platforms.${p}`)">
        <pre class="m-0 mb-2 max-h-[32rem] overflow-auto text-xs font-mono" :data-testid="`output-${p}`">{{ outputs[p] }}</pre>
        <div class="flex flex-wrap gap-2">
          <CopyButton :text="outputs[p]" />
          <NButton secondary size="small" @click="downloadText(`${name}-${p}.txt`, outputs[p], 'text/plain')">
            {{ t('tools.firewall-objects.download') }}
          </NButton>
        </div>
      </NTabPane>
    </NTabs>
  </CCard>
</template>
