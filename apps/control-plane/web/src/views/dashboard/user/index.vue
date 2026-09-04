<template>
  <div class="prototype-page grid">
    <div class="grid cols-2">
      <ui-panel variant="metric" class="hero" motion-key="subscription-usage">
        <span class="kicker">剩余流量</span>
        <b class="big"
          >{{ flowParts[0] }} <em>{{ flowParts[1] }}</em></b
        >
        <span class="muted">套餐总量 {{ quotaText }}</span>
        <div
          class="meter"
          :class="{ warn: usedPercent > 70, bad: usedPercent > 90 }"
        >
          <i :style="{ width: `${usedPercent}%` }"></i>
        </div>
        <span class="muted usage-label">已使用 {{ usedPercent }}%</span>
        <div class="hero-meta">
          <div>
            <b>{{ expireRelative }}</b
            ><span>到期时间 {{ expireDate }}</span>
          </div>
          <div>
            <b>{{ panelGroupData.nodeCount || panelGroupData.nodeNum }}</b
            ><span>可用节点</span>
          </div>
          <div v-if="resetPolicyText">
            <b>{{ resetPolicyText.title }}</b
            ><span>{{ resetPolicyText.description }}</span>
          </div>
        </div>
      </ui-panel>
      <ui-panel class="subscription-card" motion-key="subscription-export">
        <div class="card-head">
          <div>
            <span class="kicker">Subscription</span>
            <h2>订阅导出</h2>
          </div>
        </div>
        <p class="muted">选择客户端和配置模板，复制订阅地址或生成二维码。</p>
        <div class="subscription-actions">
          <button
            class="cap primary"
            type="button"
            @click="exportVisible = true"
          >
            <app-icon name="export" />打开订阅导出
          </button>
          <button
            class="cap"
            type="button"
            @click="$router.push('/node-manage/node-list')"
          >
            <app-icon name="connection" />查看我的节点
          </button>
        </div>
      </ui-panel>
    </div>
    <ui-panel motion-key="traffic-rank">
      <div class="card-head">
        <div>
          <span class="kicker">Traffic Rank</span>
          <h2>账号流量排行</h2>
        </div>
      </div>
      <traffic-table />
    </ui-panel>
    <export-node-dialog :dialog-visible-props.sync="exportVisible" />
  </div>
</template>

<script>
import { panelGroup } from '@/api/dashboard'
import { getFlow } from '@/utils/account'
import { timeStampToDate } from '@/utils'
import ExportNodeDialog from '@/views/node/list/components/ExportNodeDialog'
import TrafficTable from '@/views/dashboard/admin/compoments/TrafficTable'

export default {
  name: 'UserDashboard',
  components: { ExportNodeDialog, TrafficTable },
  data() {
    return {
      panelGroupData: {
        quota: -1,
        residualFlow: -1,
        nodeCount: 0,
        nodeNum: 0,
        expireTime: 0
      },
      exportVisible: false
    }
  },
  computed: {
    quotaText() {
      return this.panelGroupData.quota < 0
        ? '不限'
        : getFlow(this.panelGroupData.quota)
    },
    residualText() {
      return this.panelGroupData.quota < 0
        ? '不限 GB'
        : getFlow(this.panelGroupData.residualFlow)
    },
    flowParts() {
      const parts = this.residualText.split(' ')
      return [parts[0], parts[1] || '']
    },
    usedPercent() {
      const quota = Number(this.panelGroupData.quota)
      if (quota <= 0) return 0
      return Math.max(
        0,
        Math.min(
          100,
          Math.round(
            ((quota - Number(this.panelGroupData.residualFlow || 0)) / quota) *
              100
          )
        )
      )
    },
    expireDate() {
      return this.panelGroupData.expireTime
        ? timeStampToDate(this.panelGroupData.expireTime, false)
        : '—'
    },
    expireRelative() {
      if (!this.panelGroupData.expireTime) return '—'
      const days = Math.ceil(
        (this.panelGroupData.expireTime - Date.now()) / 86400000
      )
      return days >= 0 ? `${days} 天` : '已到期'
    },
    // WEB-026: consume the real system reset policy instead of asserting an
    // enabled monthly reset. Unknown/missing policy renders nothing.
    resetPolicyText() {
      const reset = this.panelGroupData.resetDownloadAndUploadMonth
      if (reset === undefined || reset === null) return null
      if (Number(reset) === 1) {
        return { title: '每月 1 日', description: '流量自动重置' }
      }
      return { title: '不自动重置', description: '流量按周期累计' }
    }
  },
  created() {
    panelGroup().then(({ data }) => {
      this.panelGroupData = data
    })
  }
}
</script>
