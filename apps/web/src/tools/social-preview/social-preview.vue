<script setup lang="ts">
import type { HttpSocial } from './social-preview.service'
import { NAlert, NButton } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { card, normalizeUrl } from './social-preview.service'

const { t } = useI18n()
const url = useQueryParam({ name: 'url', defaultValue: '' })
const { data, error, loading, run } = useApi<HttpSocial>('/v1/http/social')
const submit = () => url.value.trim() && run({ url: normalizeUrl(url.value) })
onMounted(() => {
  if (url.value)
    void submit()
})
const c = computed(() => (data.value ? card(data.value) : undefined))
const tags = computed(() => data.value
  ? [...Object.entries(data.value.og).map(([k, v]) => [`og:${k}`, v]), ...Object.entries(data.value.twitter).map(([k, v]) => [`twitter:${k}`, v])]
  : [])
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="url" :label="t('tools.social-preview.url')" placeholder="https://example.com/page" monospace test-id="url" />
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!url.trim()" data-testid="run">
        {{ t('tools.social-preview.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data && c">
    <div class="mb-4 flex flex-col gap-2" data-testid="findings">
      <NAlert v-for="p in data.problems" :key="p" type="error">
        {{ t(`tools.social-preview.findings.${p}`) }}
      </NAlert>
      <NAlert v-for="w in data.warnings" :key="w" type="warning">
        {{ t(`tools.social-preview.findings.${w}`) }}
      </NAlert>
      <NAlert v-if="!data.problems.length && !data.warnings.length" type="success">
        {{ t('tools.social-preview.ok') }}
      </NAlert>
    </div>
    <CCard :title="t('tools.social-preview.preview')" data-testid="result">
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div>
          <div class="mb-1 text-sm font-semibold">
            {{ t('tools.social-preview.large') }}
          </div>
          <div class="max-w-[500px] overflow-hidden border nt-border rounded-lg border-solid">
            <img v-if="c.image" :src="c.image" alt="" class="block aspect-[1.91/1] w-full object-cover">
            <div v-else class="aspect-[1.91/1] w-full flex items-center justify-center text-sm opacity-60">
              {{ t('tools.social-preview.noImage') }}
            </div>
            <div class="p-3">
              <div class="text-xs uppercase opacity-70">
                {{ c.site }}
              </div>
              <div class="line-clamp-2 font-semibold">
                {{ c.title || '—' }}
              </div>
              <div class="line-clamp-2 text-sm opacity-80">
                {{ c.description }}
              </div>
            </div>
          </div>
        </div>
        <div>
          <div class="mb-1 text-sm font-semibold">
            {{ t('tools.social-preview.summary') }}
          </div>
          <div class="max-w-[500px] flex overflow-hidden border nt-border rounded-lg border-solid">
            <img v-if="c.image" :src="c.image" alt="" class="block h-[100px] w-[100px] shrink-0 object-cover">
            <div v-else class="h-[100px] w-[100px] shrink-0" />
            <div class="min-w-0 p-3">
              <div class="truncate text-xs opacity-70">
                {{ c.site }}
              </div>
              <div class="truncate font-semibold">
                {{ c.title || '—' }}
              </div>
              <div class="line-clamp-2 text-sm opacity-80">
                {{ c.description }}
              </div>
            </div>
          </div>
        </div>
      </div>
      <p class="mb-0 mt-3 text-sm">
        {{ t('tools.social-preview.disclaimer') }}
      </p>
    </CCard>
    <CCard :title="t('tools.social-preview.details')">
      <dl class="grid grid-cols-1 m-0 gap-x-4 gap-y-1 text-sm md:grid-cols-[12rem_1fr]">
        <template v-for="[k, v] in tags" :key="k">
          <dt class="font-semibold font-mono">
            {{ k }}
          </dt>
          <dd class="m-0 break-all font-mono">
            {{ v }}
          </dd>
        </template>
        <template v-if="data.image">
          <dt class="font-semibold">
            {{ t('tools.social-preview.image') }}
          </dt>
          <dd class="m-0 break-all">
            {{ data.image.error ? t('tools.social-preview.imageFailed') : `HTTP ${data.image.status} · ${data.image.contentType} · ${Math.round(data.image.bytes / 1024)} KB${data.image.width ? ` · ${data.image.width}×${data.image.height}` : ''}` }}
          </dd>
        </template>
        <dt class="font-semibold">
          {{ t('tools.social-preview.canonical') }}
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ data.canonical || '—' }}
        </dd>
        <dt class="font-semibold">
          {{ t('tools.social-preview.icons') }}
        </dt>
        <dd class="m-0 break-all font-mono">
          <div v-for="i in data.icons" :key="i.href">
            {{ i.rel }}{{ i.sizes ? ` (${i.sizes})` : '' }}: {{ i.href }}
          </div>
          <span v-if="!data.icons.length">—</span>
        </dd>
        <dt class="font-semibold">
          {{ t('tools.social-preview.manifest') }}
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ data.manifest || '—' }}
        </dd>
      </dl>
      <div class="mt-3">
        <ResultActions :text="() => tags.map(([k, v]) => `${k}: ${v}`).join('\n')" :json="{ ...data, image: data.image ? { ...data.image, dataUri: '' } : undefined }" filename="social-preview.json" />
      </div>
    </CCard>
  </template>
</template>
