<script setup lang="ts">
import type { ListedRecord } from './record-list'
import { NTag } from 'naive-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from './CopyButton.vue'
import { recordZoneText } from './record-list'

/** DNS records to publish: name, type and value with copy buttons, plus the zone-file lines. */
const props = withDefaults(defineProps<{ records: ListedRecord[], zone?: boolean, ttl?: number }>(), { zone: true, ttl: 3600 })
const { t } = useI18n()
const zoneText = computed(() => recordZoneText(props.records, props.ttl))
defineExpose({ zoneText })
</script>

<template>
  <div data-testid="record-list">
    <div v-for="(r, i) in records" :key="`${r.name}-${i}`" class="mb-3" data-testid="record">
      <div class="mb-1 flex flex-wrap items-center gap-2">
        <span class="break-all text-sm font-semibold font-mono">{{ r.name }}</span>
        <NTag size="tiny" :bordered="false">
          {{ r.type ?? 'TXT' }}
        </NTag>
        <CopyButton :text="r.value" size="tiny" />
      </div>
      <p class="m-0 break-all text-sm font-mono" data-testid="record-value">
        {{ r.value }}
      </p>
    </div>
    <template v-if="zone && records.length">
      <div class="mb-2 flex items-center gap-2">
        <h3 class="m-0 text-sm font-semibold">
          {{ t('recordList.zone') }}
        </h3>
        <CopyButton :text="zoneText" size="tiny" />
      </div>
      <pre class="m-0 overflow-x-auto whitespace-pre-wrap break-all text-xs font-mono" data-testid="zone">{{ zoneText }}</pre>
    </template>
  </div>
</template>
