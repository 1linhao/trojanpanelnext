<template>
  <div class="prototype-page grid">
    <ui-panel motion-key="server-filters">
      <div class="toolbar">
        <div class="search-box">
          <app-icon name="search" /><input
            v-model="listQuery.name"
            placeholder="按名称搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <div class="search-box">
          <app-icon name="connection" /><input
            v-model="listQuery.ip"
            placeholder="按 IP 搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <div class="spacer"></div>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="downloadTemplate"
        >
          <app-icon name="document" />模板
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="handleImport"
        >
          <app-icon name="import" />导入
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="handleExport"
        >
          <app-icon name="export" />导出
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="handleBatchUpgrade"
        >
          <app-icon name="upgrade" />批量升级
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap primary small"
          type="button"
          @click="handleCreate"
        >
          <app-icon name="plus" />添加服务器
        </button>
      </div>
    </ui-panel>

    <ui-panel motion-key="server-list">
      <div class="tbl-wrap" v-liquid-loading="listLoading">
        <table class="tbl">
          <thead>
            <tr>
              <th>服务器</th>
              <th>gRPC</th>
              <th>流量配额</th>
              <th>内核</th>
              <th>Core</th>
              <th>状态</th>
              <th class="traffic-reset-column">
                <div class="traffic-reset-column-head">
                  <span>流量统计</span>
                  <button
                    v-if="checkPermission(['sysadmin', 'admin'])"
                    class="cap small traffic-reset-button"
                    type="button"
                    title="重置所有服务器流量统计"
                    aria-label="重置所有服务器流量统计"
                    @click="handleResetAllServerTraffic"
                  >
                    <app-icon name="refresh" />重置全部
                  </button>
                </div>
              </th>
              <th class="table-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!listLoading && listError" class="tbl-empty">
              <td colspan="9">{{ listError }}</td>
            </tr>
            <tr v-else-if="!listLoading && !list.length" class="tbl-empty">
              <td colspan="9">暂无数据</td>
            </tr>
            <tr v-for="row in list" :key="row.id">
              <td class="primary-cell">
                <strong>{{ row.name }}</strong
                ><small class="mono">{{ row.ip }}</small>
              </td>
              <td>
                <span class="mono muted">:{{ row.grpcPort }}</span
                ><span
                  class="chip"
                  :class="row.grpcTlsMode === 'mtls' ? 'ok' : 'warn'"
                  >{{ row.grpcTlsMode || 'legacy' }}</span
                >
              </td>
              <td class="server-actions-cell">
                <span
                  v-if="
                    !row.trafficStatus || row.trafficStatus.period === 'none'
                  "
                  class="chip plain"
                  >不限额</span
                >
                <span v-else-if="row.trafficStatus.reached" class="chip bad"
                  >已达限额</span
                >
                <template v-else>
                  <div class="traffic-label">
                    <span class="faint">{{ row.trafficStatus.period }}</span
                    ><span class="muted num">{{
                      row.trafficStatus.limitMode === 'separate'
                        ? '↑ ' +
                          quotaFlow(
                            row.trafficStatus.uploadLimit,
                            row.trafficStatus.uploadRemaining
                          ) +
                          ' / ↓ ' +
                          quotaFlow(
                            row.trafficStatus.downloadLimit,
                            row.trafficStatus.downloadRemaining
                          )
                        : getFlow(row.trafficStatus.totalRemaining)
                    }}</span>
                  </div>
                  <div class="meter">
                    <i
                      :style="{
                        width: trafficPercent(row.trafficStatus) + '%'
                      }"
                    ></i>
                  </div>
                </template>
              </td>
              <td>
                <span class="chip plain mono">{{
                  row.kernelSummary || '未上报'
                }}</span>
              </td>
              <td class="mono muted">
                {{ row.trojanPanelCoreVersion || '—' }}
              </td>
              <td>
                <span class="chip" :class="row.status === 1 ? 'ok' : 'bad'"
                  ><span class="dot"></span
                  >{{ statusComputed(row.status) }}</span
                >
              </td>
              <td>
                <button
                  v-if="checkPermission(['sysadmin', 'admin'])"
                  class="cap small traffic-reset-button"
                  type="button"
                  :disabled="resettingServerId !== 0"
                  :title="`重置服务器 ${row.name} 的流量统计`"
                  :aria-label="`重置服务器 ${row.name} 的流量统计`"
                  @click="handleResetServerTraffic(row)"
                >
                  <app-icon name="refresh-left" />重置流量
                </button>
                <span v-else class="faint">—</span>
              </td>
              <td>
                <div class="row-actions">
                  <button
                    class="icon-btn"
                    type="button"
                    title="运行状态"
                    @click="handleDetail(row)"
                  >
                    <app-icon name="view" />
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    class="icon-btn"
                    type="button"
                    title="内核管理"
                    @click="handleKernelManage(row)"
                  >
                    <app-icon name="top" />
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    class="icon-btn"
                    type="button"
                    title="编辑"
                    @click="handleUpdate(row)"
                  >
                    <app-icon name="edit" />
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    class="icon-btn danger"
                    type="button"
                    title="删除"
                    @click="handleDelete(row)"
                    :disabled="deletingServerId !== 0"
                  >
                    <app-icon name="delete" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <pagination
        v-if="total > 0"
        :total="total"
        :page.sync="listQuery.pageNum"
        :limit.sync="listQuery.pageSize"
        @pagination="getList"
      />
    </ui-panel>

    <ui-sheet
      v-if="detailServer"
      motion-role="shared"
      :motion-key="'server-' + detailServer.id"
    >
      <div class="card-head">
        <div>
          <span class="kicker">Server State</span>
          <h2>{{ detailServer.name }} · 运行状态</h2>
        </div>
        <button
          class="icon-btn"
          type="button"
          aria-label="关闭服务器详情"
          @click="detailServer = null"
        >
          <app-icon name="close" />
        </button>
      </div>
      <div class="server-detail-layout">
        <div class="rings-row">
          <div
            v-for="ring in [
              {
                label: 'CPU',
                value: detailState.cpuUsed || 0,
                color: 'var(--chart-1)'
              },
              {
                label: '内存',
                value: detailState.memUsed || 0,
                color: 'var(--chart-2)'
              },
              {
                label: '磁盘',
                value: detailState.diskUsed || 0,
                color: 'var(--chart-3)'
              }
            ]"
            :key="ring.label"
          >
            <div
              class="ring"
              :style="{ '--p': ring.value, '--ring-color': ring.color }"
            >
              <b>{{ ring.value }}<em>%</em></b>
            </div>
            <div class="ring-label">{{ ring.label }}</div>
          </div>
        </div>
        <div class="kv-grid">
          <div class="kv">
            <span>传输安全</span
            ><b>{{ detailServer.grpcTlsMode || 'legacy' }}</b>
          </div>
          <div class="kv">
            <span>Core 版本</span
            ><b>{{ detailServer.trojanPanelCoreVersion || '—' }}</b>
          </div>
          <div class="kv">
            <span>创建时间</span
            ><b>{{ timeStampToDate(detailServer.createTime, false) }}</b>
          </div>
          <div class="kv">
            <span>运行状态</span
            ><b>{{ statusComputed(detailServer.status) }}</b>
          </div>
        </div>
      </div>
    </ui-sheet>

    <ui-dialog
      v-if="deleteServer"
      :visible="true"
      title="删除节点服务器"
      width="580px"
      role="alertdialog"
      described-by="server-delete-description"
      custom-class="server-delete-dialog"
      @open="$refs.cancelDelete.$el.focus()"
      @close="deleteServer = null"
    >
      <div id="server-delete-description" class="server-delete-description">
        <p class="server-delete-target">
          确认删除服务器「{{ deleteServer.name }}」？
        </p>
        <p>
          两种方式都会先卸载目标机器的容器与镜像，再移除 Web
          中的服务器及关联代理节点。
        </p>
        <p>
          <strong>删除：</strong>保留目标机器的数据、证书、伪装站和部署配置。
        </p>
        <p class="server-delete-warning">
          <strong>彻底删除：</strong
          >同时删除目标机器的服务数据、证书、伪装站和部署配置，并清理 Web
          中关联的流量、内核任务和连接信息。此操作无法恢复。
        </p>
      </div>
      <div slot="footer" class="dialog-footer server-delete-actions">
        <liquid-button ref="cancelDelete" @click="deleteServer = null"
          >取消</liquid-button
        >
        <liquid-button type="primary" @click="confirmDelete(false)"
          >删除</liquid-button
        >
        <liquid-button type="danger" @click="confirmDelete(true)"
          >彻底删除</liquid-button
        >
      </div>
    </ui-dialog>

    <NodeServerForm
      ref="nodeServerForm"
      :node-server="temp"
      :dialog-status="dialogStatus"
      :dialog-visible.sync="dialogFormVisible"
      :get-list="getList"
    />
    <import-tip
      ref="importTip"
      :dialog-form-visible.sync="importVisible"
      :label="$t('table.coverByNodeServerName')"
      :import-data="importData"
      :download-template="downloadTemplate"
    />
  </div>
</template>

<script>
import { timeStampToDate } from '@/utils'
import Pagination from '@/components/Pagination'
import ImportTip from '@/components/ImportTip'
import latestListRequest from '@/mixins/latest-list-request'
import { MessageBox } from '@/utils/liquid-feedback'
import checkPermission from '@/utils/permission'
import {
  deleteNodeServerById,
  exportNodeServer,
  importNodeServer,
  selectNodeServerPage,
  nodeServerState,
  resetNodeServerTraffic
} from '@/api/node-server'
import NodeServerForm from '@/views/node-server/list/compoments/NodeServerForm'
import { downloadTemplate } from '@/api/file-task'
import { getFlow } from '@/utils/account'

export default {
  name: 'NodeServer',
  components: { NodeServerForm, Pagination, ImportTip },
  mixins: [latestListRequest],
  data() {
    return {
      tableKey: 0,
      listLoading: true,
      listError: '',
      list: null,
      total: 0,
      listQuery: {
        pageNum: 1,
        pageSize: 20,
        ip: undefined,
        name: undefined
      },
      temp: {
        id: undefined,
        ip: '',
        name: '',
        grpcPort: 8100,
        grpcTlsMode: 'mtls',
        grpcTlsServerName: '',
        trafficPeriod: 'none',
        trafficLimitMode: 'combined',
        trafficTotalLimit: 0,
        trafficUploadLimit: 0,
        trafficDownloadLimit: 0,
        trojanPanelCoreVersion: '',
        createTime: new Date()
      },
      dialogFormVisible: false,
      textMap: {
        update: this.$t('table.edit'),
        create: this.$t('table.add')
      },
      importVisible: false,
      dialogStatus: '',
      resettingServerId: 0,
      deletingServerId: 0,
      deleteServer: null,
      detailServer: null,
      detailState: { cpuUsed: 0, memUsed: 0, diskUsed: 0 }
    }
  },
  created() {
    this.getList()
  },
  filters: {
    statusTypeFilter(status) {
      return status > 0 ? 'success' : 'danger'
    },
    disabledFilter(status) {
      return status !== 1
    }
  },
  computed: {
    statusComputed() {
      return function (status) {
        return status === 1
          ? this.$t('table.nodeStatusSuccess')
          : this.$t('table.nodeStatusError')
      }
    }
  },
  methods: {
    getFlow,
    quotaFlow(limit, remaining) {
      return limit > 0 ? getFlow(remaining) : this.$t('traffic.unlimited')
    },
    // WEB-015: real usage ratio instead of a fixed demo value. Combined mode
    // uses total used/limit; separate mode uses the dominant direction. A
    // missing or zero limit (unreported / unlimited) yields null so the bar
    // renders empty rather than a fake percentage.
    trafficPercent(trafficStatus) {
      if (!trafficStatus) return 0
      if (trafficStatus.limitMode === 'separate') {
        const ratios = [
          [trafficStatus.uploadUsed, trafficStatus.uploadLimit],
          [trafficStatus.downloadUsed, trafficStatus.downloadLimit]
        ]
          .filter(([, limit]) => Number(limit) > 0)
          .map(([used, limit]) => Number(used || 0) / Number(limit))
        return ratios.length
          ? Math.min(100, Math.round(Math.max(...ratios) * 100))
          : 0
      }
      const limit = Number(trafficStatus.totalLimit || 0)
      return limit > 0
        ? Math.min(
            100,
            Math.round((Number(trafficStatus.totalUsed || 0) / limit) * 100)
          )
        : 0
    },
    checkPermission,
    timeStampToDate,
    getList() {
      const request = this.beginListRequest()
      return selectNodeServerPage(this.listQuery)
        .then((response) => {
          if (!this.ownsListRequest(request)) return
          this.list = response.data.nodeServers
          this.total = response.data.total
        })
        .catch(() => {
          if (!this.ownsListRequest(request)) return
          this.list = []
          this.total = 0
          this.listError = '请求失败，请重试'
        })
        .finally(() => this.finishListRequest(request))
    },
    resetTemp() {
      this.temp = {
        id: undefined,
        ip: '',
        name: '',
        grpcPort: 8100,
        grpcTlsMode: 'mtls',
        grpcTlsServerName: '',
        trafficPeriod: 'none',
        trafficLimitMode: 'combined',
        trafficTotalLimit: 0,
        trafficUploadLimit: 0,
        trafficDownloadLimit: 0,
        trojanPanelCoreVersion: '',
        createTime: new Date()
      }
    },
    handleFilter() {
      this.listQuery.pageNum = 1
      this.getList()
    },
    handleCreate() {
      this.resetTemp()
      this.dialogStatus = 'create'
      this.dialogFormVisible = true
      this.$refs.nodeServerForm.clearValidate()
    },
    handleDelete(row) {
      if (this.deletingServerId || !checkPermission(['sysadmin'])) return
      this.deleteServer = row
    },
    async confirmDelete(purge) {
      const row = this.deleteServer
      if (!row || this.deletingServerId || !checkPermission(['sysadmin']))
        return
      this.deletingServerId = row.id
      this.deleteServer = null
      try {
        const response = await deleteNodeServerById({ id: row.id, purge })
        await this.getList()
        const pending = response.data && response.data.cleanupPending
        this.$notify({
          title: pending ? '清理待完成' : 'Success',
          message: pending
            ? '目标机器已卸载，Web 记录已删除；维护服务正在完成最终清理，异常时会自动重试。'
            : this.$t('confirm.deleteSuccess'),
          type: pending ? 'info' : 'success',
          duration: pending ? 6000 : 2000
        })
      } catch (_) {
        // The request interceptor reports the server error. Keep the row so
        // the administrator can fix connectivity or dependencies and retry.
      } finally {
        this.deletingServerId = 0
      }
    },
    handleResetAllServerTraffic() {
      this.showPendingTrafficReset('全部服务器')
    },
    handleResetServerTraffic(row) {
      if (this.resettingServerId) return
      MessageBox.confirm(
        this.$t('traffic.resetServerConfirm', { name: row.name }),
        this.$t('confirm.warn'),
        {
          confirmButtonText: this.$t('confirm.yes'),
          cancelButtonText: this.$t('confirm.cancel'),
          type: 'warning'
        }
      )
        .then(() => {
          this.resettingServerId = row.id
          return resetNodeServerTraffic({ id: row.id })
        })
        .then(() => {
          this.$notify({
            title: 'Success',
            message: this.$t('traffic.resetServerSuccess'),
            type: 'success',
            duration: 2000
          })
          this.getList()
        })
        .finally(() => {
          this.resettingServerId = 0
        })
    },
    showPendingTrafficReset(target) {
      this.$message({
        type: 'info',
        showClose: true,
        message: `${target}流量统计重置接口待接入，当前未修改任何数据`
      })
    },
    handleUpdate(row) {
      this.temp = Object.assign(this.temp, row)
      this.dialogStatus = 'update'
      this.dialogFormVisible = true
      this.$refs.nodeServerForm.clearValidate()
    },
    handleDetail(row) {
      if (this.detailServer && this.detailServer.id === row.id) {
        this.detailServer = null
        return
      }
      nodeServerState({ id: row.id }).then(({ data }) => {
        this.detailServer = row
        this.detailState = data
      })
    },
    handleKernelManage(row) {
      this.$router.push({
        path: 'kernel-upgrade',
        query: { serverId: row.id }
      })
    },
    handleBatchUpgrade() {
      this.$router.push({ path: 'kernel-upgrade' })
    },
    async importData(params) {
      const valid = await this.$refs.importTip.$refs.dataForm.validate()
      if (!valid) return false
      const formData = new FormData()
      formData.append('file', params.file)
      formData.append('cover', this.$refs.importTip.temp.cover)
      await importNodeServer(formData)
      this.importVisible = false
      this.$notify({
        title: 'Success',
        message: this.$t('confirm.taskSubmitSuccess'),
        type: 'success',
        duration: 2000
      })
      return true
    },
    handleImport() {
      this.importVisible = true
    },
    handleExport() {
      exportNodeServer().then(() => {
        this.importVisible = false
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.taskSubmitSuccess'),
          type: 'success',
          duration: 2000
        })
      })
    },
    downloadTemplate() {
      downloadTemplate({ id: 2 }).then((res) => {
        // 将二进制文件转化为可访问的url
        const blob = new Blob([res.data], {
          type: 'application/octet-stream'
        })
        let url = window.URL.createObjectURL(blob)
        let a = document.createElement('a')
        document.body.appendChild(a)
        a.href = url
        let dis = res.headers['content-disposition']
        a.download = dis.split('attachment; filename=')[1]
        // 模拟点击下载
        a.click()
        window.URL.revokeObjectURL(url)
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.taskDownloadSuccess'),
          type: 'success',
          duration: 2000
        })
      })
    }
  }
}
</script>

<style scoped>
.liquid-button {
  margin-left: 10px;
}
.server-delete-description {
  line-height: 1.7;
}
.server-delete-description p {
  margin: 0 0 12px;
}
.server-delete-target {
  overflow-wrap: anywhere;
}
.server-delete-warning {
  color: var(--bad-fg);
}
.server-delete-actions .liquid-button {
  margin-left: 0;
}
</style>
