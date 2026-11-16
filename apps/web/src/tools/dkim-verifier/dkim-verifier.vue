<script setup lang="ts">
import type { EmailDkimVerify } from './dkim-verifier.service'
import { NAlert, NButton, NFormItem, NInput, NTag } from 'naive-ui'
import { computed, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError as ApiErr, apiPost } from '@/api/client'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import CCard from '@/ui/c-card.vue'
import { byteLength, looksLikeMessage, MAX_MESSAGE } from './dkim-verifier.service'

const { t } = useI18n()
// The message is personal data: kept in memory, sent in a POST body, never put in the URL.
const text = ref('')
const size = computed(() => byteLength(text.value))
const tooBig = computed(() => size.value > MAX_MESSAGE)
const data = shallowRef<EmailDkimVerify>()
const error = shallowRef<ApiErr>()
const loading = ref(false)
async function submit() {
  if (!text.value.trim() || tooBig.value)
    return
  loading.value = true
  error.value = undefined
  try {
    data.value = await apiPost<EmailDkimVerify>('/v1/email/dkim-verify', { message: text.value })
  }
  catch (e) {
    data.value = undefined
    error.value = e instanceof ApiErr ? e : new ApiErr('NETWORK_ERROR', String(e))
  }
  finally {
    loading.value = false
  }
}
const fileInput = ref<HTMLInputElement>()
async function loadFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f)
    text.value = await f.text()
  ;(e.target as HTMLInputElement).value = ''
}
const resultType = { pass: 'success', fail: 'error', permerror: 'error', temperror: 'warning' } as const
const passAligned = computed(() => data.value?.signatures.some(s => s.result === 'pass' && s.fromAligned))
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <NFormItem :label="t('tools.dkim-verifier.message')" :validation-status="tooBig ? 'error' : undefined" :feedback="tooBig ? t('tools.dkim-verifier.tooBig') : undefined">
        <NInput v-model:value="text" type="textarea" :autosize="{ minRows: 8, maxRows: 20 }" class="font-mono" :placeholder="t('tools.dkim-verifier.placeholder')" :input-props="{ 'aria-label': t('tools.dkim-verifier.message'), 'spellcheck': false }" data-testid="message" />
      </NFormItem>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.dkim-verifier.hint') }}
      </p>
      <NAlert v-if="text.trim() && !looksLikeMessage(text)" type="warning" class="mb-3">
        {{ t('tools.dkim-verifier.notRaw') }}
      </NAlert>
      <div class="flex flex-wrap gap-3">
        <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!text.trim() || tooBig" data-testid="run">
          {{ t('tools.dkim-verifier.run') }}
        </NButton>
        <NButton secondary @click="fileInput?.click()">
          {{ t('tools.dkim-verifier.open') }}
        </NButton>
        <input ref="fileInput" type="file" accept=".eml,message/rfc822,text/plain" class="hidden" :aria-label="t('tools.dkim-verifier.open')" @change="loadFile">
      </div>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <div class="mb-4 flex flex-col gap-2" data-testid="verdict">
      <NAlert v-for="p in data.problems" :key="p" :type="p === 'no-aligned-pass' ? 'warning' : 'error'">
        {{ t(`tools.dkim-verifier.problems.${p}`) }}
      </NAlert>
      <NAlert v-if="passAligned" type="success">
        {{ t('tools.dkim-verifier.ok') }}
      </NAlert>
    </div>
    <CCard v-for="(s, i) in data.signatures" :key="i" :title="t('tools.dkim-verifier.signature', { n: i + 1, domain: s.domain })" data-testid="result">
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <NTag :type="resultType[s.result]" :bordered="false" data-testid="sig-result">
          {{ t(`tools.dkim-verifier.results.${s.result}`) }}
        </NTag>
        <NTag :type="s.fromAligned ? 'success' : 'warning'" :bordered="false">
          {{ s.fromAligned ? t('tools.dkim-verifier.aligned') : t('tools.dkim-verifier.notAligned') }}
        </NTag>
      </div>
      <dl class="grid grid-cols-1 m-0 gap-x-4 gap-y-1 text-sm md:grid-cols-[10rem_1fr]">
        <dt class="font-semibold">
          {{ t('tools.dkim-verifier.selector') }}
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ s.selector }}._domainkey.{{ s.domain }}
        </dd>
        <dt class="font-semibold">
          {{ t('tools.dkim-verifier.algorithm') }}
        </dt>
        <dd class="m-0 font-mono">
          {{ s.algorithm }} · c={{ s.canon }}
        </dd>
        <dt class="font-semibold">
          {{ t('tools.dkim-verifier.headers') }}
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ s.headers.join(', ') || '—' }}
        </dd>
        <template v-if="s.signed">
          <dt class="font-semibold">
            {{ t('tools.dkim-verifier.signed') }}
          </dt>
          <dd class="m-0 font-mono">
            {{ s.signed }}{{ s.expires ? ` → ${s.expires}` : '' }}
          </dd>
        </template>
        <template v-if="s.error">
          <dt class="font-semibold">
            {{ t('tools.dkim-verifier.error') }}
          </dt>
          <dd class="m-0 break-all font-mono">
            {{ s.error }}
          </dd>
        </template>
        <template v-if="s.keyRecord">
          <dt class="font-semibold">
            {{ t('tools.dkim-verifier.key') }}
          </dt>
          <dd class="m-0 break-all font-mono">
            {{ s.keyRecord }}
          </dd>
        </template>
      </dl>
      <NAlert v-for="w in s.warnings" :key="w" type="warning" class="mt-2">
        {{ t(`tools.dkim-verifier.warnings.${w}`) }}
      </NAlert>
    </CCard>
    <CCard v-if="data.authResults.length || data.arc.length" :title="t('tools.dkim-verifier.receivers')">
      <p class="mt-0 text-sm">
        {{ t('tools.dkim-verifier.receiversHint') }}
      </p>
      <div v-for="(a, i) in data.authResults" :key="`ar${i}`" class="mb-2 break-all text-sm font-mono">
        {{ a }}
      </div>
      <div v-for="a in data.arc" :key="`arc${a.instance}`" class="mb-2 break-all text-sm font-mono">
        ARC i={{ a.instance }} cv={{ a.cv }} d={{ a.domain }} — {{ a.results }}
      </div>
    </CCard>
    <ResultActions :text="() => data!.signatures.map(s => `${s.result}\t${s.domain}\t${s.selector}\t${s.error}`).join('\n')" :json="data" filename="dkim-verify.json" />
  </template>
</template>
