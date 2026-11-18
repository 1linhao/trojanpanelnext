<template>
  <ui-panel variant="metric" class="node-card">
    <div class="node-top">
      <div class="grow">
        <h3 :title="summary.serverName">{{ summary.serverName }}</h3>
        <span class="muted">{{ $t('trafficDisplay.summary') }} · {{ $t(periodLabel) }}</span>
      </div>
    </div>
    <div class="node-meta">
      <span class="chip plain">{{ $t('trafficDisplay.used', { traffic: getFlow(summary.totalUsed) }) }}</span>
      <span class="chip plain">{{ $t('trafficDisplay.limit', { traffic: quotaFlow(totalLimit) }) }}</span>
    </div>
    <div v-if="summary.upload && summary.download" class="node-meta">
      <span class="chip plain">{{ directionText('upload') }}</span>
      <span class="chip plain">{{ directionText('download') }}</span>
    </div>
  </ui-panel>
</template>

<script>
import { getFlow } from '@/utils/account'

export default {
  name: 'ServerTrafficSummaryCard',
  props: {
    summary: { type: Object, required: true }
  },
  computed: {
    periodLabel() {
      return this.summary.period === 'none' ? 'traffic.total' : `traffic.${this.summary.period}`
    },
    totalLimit() {
      const { upload, download, totalLimit } = this.summary
      if (!upload || !download) return totalLimit
      return upload.limit > 0 && download.limit > 0 ? upload.limit + download.limit : 0
    }
  },
  methods: {
    getFlow,
    quotaFlow(limit) {
      return limit > 0 ? getFlow(limit) : this.$t('traffic.unlimited')
    },
    directionText(direction) {
      const { used, limit } = this.summary[direction]
      return this.$t(`trafficDisplay.${direction}`, {
        used: getFlow(used),
        limit: this.quotaFlow(limit)
      })
    }
  }
}
</script>
