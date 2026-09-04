<template>
  <div class="app-container">
    <ui-panel
      v-if="batchMode"
      class="section"
      motion-key="kernel-batch-upgrade"
    >
      <div class="section-head">{{ $t('kernel.batchUpgrade') }}</div>
      <liquid-form label-width="150px">
        <liquid-form-item :label="$t('kernel.nodes')">
          <liquid-select
            v-model="selectedNodeIds"
            multiple
            filterable
          >
            <option
              v-for="server in servers"
              :key="server.id"
              :label="server.name"
              :value="server.id"
            />
          </liquid-select>
        </liquid-form-item>
        <liquid-form-item :label="$t('kernel.canary')">
          <liquid-select v-model="canaryNodeId" clearable>
            <option
              v-for="server in selectedServers"
              :key="server.id"
              :label="server.name"
              :value="server.id"
            />
          </liquid-select>
        </liquid-form-item>
      </liquid-form>
    </ui-panel>

    <ui-panel
      v-if="!batchMode"
      v-liquid-loading="inventoryLoading"
      class="section"
      motion-key="kernel-inventory"
    >
      <div class="section-head">
        {{ $t('kernel.inventory') }}
        <liquid-button size="sm" class="align-action-right" @click="loadInventory">
          {{ $t('kernel.refreshInventory') }}
        </liquid-button>
      </div>
      <liquid-descriptions v-if="currentServer.id && !inventoryUnavailable" :column="2" border>
        <liquid-descriptions-item :label="$t('kernel.platform')">
          {{ inventory ? inventory.os + '/' + inventory.arch : '-' }}
        </liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('kernel.transport')">
          {{ currentServer.grpcTlsMode || 'legacy' }}
          <liquid-button
            v-if="currentServer.grpcTlsMode !== 'mtls'"
            type="warning"
            size="sm"
            @click="enableMTLS"
          >
            {{ $t('kernel.probeMTLS') }}
          </liquid-button>
        </liquid-descriptions-item>
      </liquid-descriptions>
      <p v-else-if="currentServer.id && inventoryUnavailable && !inventoryLoading" class="node-grid__empty">内核清单不可用</p>
    </ui-panel>

    <ui-panel class="section" motion-key="managed-kernels">
      <div class="section-head">
        {{ $t('kernel.managedKernels') }}
        <liquid-button size="sm" class="align-action-right" @click="loadReleases(true)">
          {{ $t('kernel.refreshReleases') }}
        </liquid-button>
      </div>
      <liquid-table :data="kernelRows" border>
        <liquid-table-column prop="name" :label="$t('kernel.kernel')" width="130" />
        <liquid-table-column :label="$t('kernel.currentVersion')" min-width="140">
          <template slot-scope="{ row }">{{
            row.currentVersion || '-'
          }}</template>
        </liquid-table-column>
        <liquid-table-column :label="$t('kernel.channel')" width="150">
          <template slot-scope="{ row }">
            <liquid-select
              v-model="row.targetChannel"
              size="sm"
              @change="row.targetVersion = ''"
            >
              <option :label="$t('kernel.stable')" value="stable" />
              <option :label="$t('kernel.prerelease')" value="prerelease" />
            </liquid-select>
          </template>
        </liquid-table-column>
        <liquid-table-column :label="$t('kernel.targetVersion')" min-width="180">
          <template slot-scope="{ row }">
            <liquid-select v-model="row.targetVersion" size="sm" filterable>
              <option
                v-for="release in releases[row.key][row.targetChannel]"
                :key="release.version"
                :label="release.version"
                :value="release.version"
              />
            </liquid-select>
          </template>
        </liquid-table-column>
        <liquid-table-column :label="$t('kernel.sha256')" min-width="220">
          <template slot-scope="{ row }">
            <span class="hash">{{ row.currentSha256 || '-' }}</span>
          </template>
        </liquid-table-column>
        <liquid-table-column :label="$t('kernel.usage')" width="100">
          <template slot-scope="{ row }">
            <liquid-tag :type="row.inUse ? 'success' : 'info'">
              {{ row.inUse ? $t('kernel.inUse') : $t('kernel.notInUse') }}
            </liquid-tag>
          </template>
        </liquid-table-column>
        <liquid-table-column
          v-if="!batchMode"
          :label="$t('kernel.rollback')"
          min-width="220"
        >
          <template slot-scope="{ row }">
            <liquid-button
              v-for="version in row.rollbackVersions"
              :key="version.version"
              size="sm"
              @click="
                submitSingle(
                  row,
                  version.version,
                  version.channelName,
                  'rollback'
                )
              "
            >
              {{ version.version }}
            </liquid-button>
          </template>
        </liquid-table-column>
        <liquid-table-column :label="$t('table.actions')" width="120">
          <template slot-scope="{ row }">
            <liquid-button
              type="primary"
              size="sm"
              :disabled="!row.targetVersion || !row.supported"
              @click="
                batchMode
                  ? toggleTarget(row)
                  : submitSingle(
                      row,
                      row.targetVersion,
                      row.targetChannel,
                      'install'
                    )
              "
            >
              {{
                batchMode && row.selected
                  ? $t('kernel.selected')
                  : $t('kernel.upgrade')
              }}
            </liquid-button>
          </template>
        </liquid-table-column>
      </liquid-table>
      <liquid-button
        v-if="batchMode"
        type="primary"
        class="submit"
        :disabled="!canSubmitBatch"
        @click="submitBatch"
      >
        {{ $t('kernel.createBatchTask') }}
      </liquid-button>
    </ui-panel>

    <ui-panel
      v-for="task in tasks"
      :key="task.id"
      class="section"
      :motion-key="`kernel-task-${task.id}`"
    >
      <div class="section-head">
        {{ $t('kernel.task') }} #{{ task.id }}
        <liquid-tag>{{ statusLabel(task.status) }}</liquid-tag>
      </div>
      <liquid-table :data="task.items || []" border>
        <liquid-table-column prop="nodeServerName" :label="$t('kernel.node')" />
        <liquid-table-column prop="kernel" :label="$t('kernel.kernel')" />
        <liquid-table-column prop="fromVersion" :label="$t('kernel.fromVersion')" />
        <liquid-table-column
          prop="targetVersion"
          :label="$t('kernel.targetVersion')"
        />
        <liquid-table-column :label="$t('kernel.stage')">
          <template slot-scope="{ row }">{{ stageLabel(row.stage) }}</template>
        </liquid-table-column>
        <liquid-table-column
          prop="error"
          :label="$t('kernel.error')"
          min-width="220"
        />
      </liquid-table>
      <liquid-button
        v-if="task.status === 'failed' || task.status === 'partial'"
        type="warning"
        class="submit"
        @click="retryTask(task)"
      >
        {{ $t('kernel.retryFailed') }}
      </liquid-button>
    </ui-panel>

    <ui-panel class="section" motion-key="kernel-task-history">
      <div class="section-head">{{ $t('kernel.taskHistory') }}</div>
      <liquid-table :data="taskHistory" border>
        <liquid-table-column prop="id" label="ID" width="90" />
        <liquid-table-column prop="operatorName" :label="$t('kernel.operator')" />
        <liquid-table-column :label="$t('table.status')">
          <template slot-scope="{ row }">{{
            statusLabel(row.status)
          }}</template>
        </liquid-table-column>
        <liquid-table-column :label="$t('table.createTime')">
          <template slot-scope="{ row }">{{
            formatTaskTime(row.createdAt)
          }}</template>
        </liquid-table-column>
        <liquid-table-column :label="$t('table.actions')" width="100">
          <template slot-scope="{ row }">
            <liquid-button size="sm" @click="openTask(row.id)">
              {{ $t('kernel.viewTask') }}
            </liquid-button>
          </template>
        </liquid-table-column>
      </liquid-table>
    </ui-panel>
  </div>
</template>

<script>
import { MessageBox } from '@/utils/liquid-feedback'
import { timeStampToDate } from '@/utils'
import { selectNodeServerList, selectNodeServerById } from '@/api/node-server'
import {
  createKernelTask,
  kernelInventory,
  kernelReleases,
  probeKernelMTLS,
  retryKernelTask,
  selectKernelTaskById,
  selectKernelTaskPage
} from '@/api/kernel-upgrade'

const terminal = ['succeeded', 'partial', 'failed']

export default {
  name: 'KernelUpgrade',
  data() {
    return {
      servers: [],
      selectedNodeIds: [],
      canaryNodeId: undefined,
      currentServer: {},
      inventory: null,
      inventoryLoading: false,
      inventoryUnavailable: false,
      tasks: [],
      taskHistory: [],
      runningTaskIds: [],
      timer: null,
      releases: {
        xray: { stable: [], prerelease: [] },
        hysteria2: { stable: [], prerelease: [] }
      },
      rows: [
        {
          key: 'xray',
          name: 'Xray',
          targetChannel: 'stable',
          targetVersion: '',
          selected: false
        },
        {
          key: 'hysteria2',
          name: 'Hysteria2',
          targetChannel: 'stable',
          targetVersion: '',
          selected: false
        }
      ]
    }
  },
  computed: {
    serverId() {
      return Number(this.$route.query.serverId || 0)
    },
    batchMode() {
      return this.serverId === 0
    },
    selectedServers() {
      return this.servers.filter((server) =>
        this.selectedNodeIds.includes(server.id)
      )
    },
    kernelRows() {
      return this.rows.map((row) => {
        const inventoryItem = this.inventory
          ? (this.inventory.kernels || []).find(
              (item) => item.kernel === (row.key === 'xray' ? 1 : 2)
            )
          : null
        const versions = inventoryItem ? inventoryItem.versions || [] : []
        return Object.assign(row, {
          supported:
            this.batchMode || (inventoryItem && inventoryItem.supported),
          currentVersion: inventoryItem ? inventoryItem.currentVersion : '',
          currentSha256: inventoryItem ? inventoryItem.currentSha256 : '',
          inUse: inventoryItem ? inventoryItem.inUse : false,
          rollbackVersions: versions
            .filter(
              (version) =>
                version.successful &&
                version.version !== inventoryItem.currentVersion
            )
            .slice(0, 2)
            .map((version) =>
              Object.assign(version, {
                channelName: this.channelName(version.channel)
              })
            )
        })
      })
    },
    canSubmitBatch() {
      return (
        this.selectedNodeIds.length > 0 &&
        this.rows.some((row) => row.selected && row.targetVersion)
      )
    }
  },
  created() {
    selectNodeServerList().then((response) => {
      this.servers = response.data
      if (this.serverId) {
        this.selectedNodeIds = [this.serverId]
      }
    })
    if (this.serverId) {
      selectNodeServerById({ id: this.serverId }).then((response) => {
        this.currentServer = response.data
      })
      this.loadInventory()
    }
    this.loadReleases(false)
    this.loadTaskHistory().then(() => this.loadInitialTasks())
  },
  beforeDestroy() {
    clearTimeout(this.timer)
  },
  methods: {
    // WEB-029: raw/missing timestamps render a stable placeholder, not the raw value.
    formatTaskTime(createdAt) {
      if (createdAt === undefined || createdAt === null || createdAt === '') return '—'
      return timeStampToDate(createdAt, true)
    },
    stageLabel(stage) {
      return this.$t(`kernel.stages.${stage}`)
    },
    statusLabel(status) {
      return this.$t(`kernel.statuses.${status}`)
    },
    loadTaskHistory() {
      return Promise.all([
        selectKernelTaskPage({ pageNum: 1, pageSize: 20 }),
        selectKernelTaskPage({ pageNum: 1, pageSize: 100, status: 'queued' }),
        selectKernelTaskPage({ pageNum: 1, pageSize: 100, status: 'running' })
      ]).then(([historyResponse, queuedResponse, runningResponse]) => {
        this.taskHistory = historyResponse.data.tasks || []
        const activeTasks = [
          ...(queuedResponse.data.tasks || []),
          ...(runningResponse.data.tasks || [])
        ]
        this.runningTaskIds = [...new Set(activeTasks.map((task) => task.id))]
      })
    },
    routeTaskIds() {
      const value = this.$route.query.taskIds || this.$route.query.taskId || ''
      return [
        ...new Set(
          String(value)
            .split(',')
            .map(Number)
            .filter((id) => id > 0)
        )
      ]
    },
    activeTaskIds() {
      return this.runningTaskIds
    },
    loadInitialTasks() {
      const ids = [
        ...new Set([...this.routeTaskIds(), ...this.activeTaskIds()])
      ]
      if (ids.length) this.loadTasks(ids)
    },
    rememberTasks(ids) {
      const remembered = [
        ...new Set([...this.tasks.map((task) => task.id), ...ids])
      ]
      const query = Object.assign({}, this.$route.query, {
        taskIds: remembered.join(',')
      })
      delete query.taskId
      this.$router.replace({ path: this.$route.path, query }).catch(() => {})
    },
    openTask(id) {
      this.rememberTasks([id])
      this.loadTasks([id])
    },
    channelName(channel) {
      return channel === 2 ? 'prerelease' : channel === 3 ? 'legacy' : 'stable'
    },
    loadInventory() {
      this.inventoryLoading = true
      this.inventoryUnavailable = false
      return kernelInventory({ nodeServerId: this.serverId })
        .then((response) => {
          this.inventory = response.data
        })
        .catch(() => {
          this.inventory = null
          this.inventoryUnavailable = true
        })
        .finally(() => {
          this.inventoryLoading = false
        })
    },
    loadReleases(refresh) {
      const calls = []
      ;['xray', 'hysteria2'].forEach((kernel) => {
        ['stable', 'prerelease'].forEach((channel) => {
          calls.push(
            kernelReleases({ kernel, channel, refresh }).then((response) => {
              this.releases[kernel][channel] = response.data.releases || []
            })
          )
        })
      })
      return Promise.all(calls)
    },
    toggleTarget(row) {
      row.selected = !row.selected
    },
    confirmPrerelease(targets) {
      if (!targets.some((target) => target.channel === 'prerelease'))
        return Promise.resolve()
      return MessageBox.confirm(
        this.$t('kernel.prereleaseWarning'),
        this.$t('confirm.warn'),
        {
          type: 'warning'
        }
      ).then(() =>
        MessageBox.confirm(
          this.$t('kernel.prereleaseSecondWarning'),
          this.$t('confirm.warn'),
          {
            type: 'error'
          }
        )
      )
    },
    submitSingle(row, version, channel, action) {
      const targets = [{ kernel: row.key, version, channel, action }]
      this.confirmPrerelease(targets).then(() =>
        this.submit([this.serverId], 0, targets)
      )
    },
    submitBatch() {
      const targets = this.rows
        .filter((row) => row.selected)
        .map((row) => ({
          kernel: row.key,
          version: row.targetVersion,
          channel: row.targetChannel,
          action: 'install'
        }))
      this.confirmPrerelease(targets).then(() =>
        this.submit(this.selectedNodeIds, this.canaryNodeId || 0, targets)
      )
    },
    submit(nodeServerIds, canaryNodeServerId, targets) {
      return createKernelTask({
        nodeServerIds,
        canaryNodeServerId,
        targets
      }).then((response) => {
        const task = response.data
        this.upsertTask(task)
        this.rememberTasks([task.id])
        this.loadTasks([task.id])
      })
    },
    upsertTask(task) {
      const index = this.tasks.findIndex((item) => item.id === task.id)
      if (index === -1) {
        this.tasks.push(task)
      } else {
        this.tasks.splice(index, 1, task)
      }
      this.tasks.sort((left, right) => right.id - left.id)
    },
    loadTasks(ids) {
      clearTimeout(this.timer)
      const taskIds = [
        ...new Set([...this.tasks.map((task) => task.id), ...ids])
      ]
      return Promise.all(
        taskIds.map((id) =>
          selectKernelTaskById({ id })
            .then((response) => response.data)
            .catch(() => null)
        )
      ).then((loadedTasks) => {
        let completed = false
        loadedTasks.filter(Boolean).forEach((task) => {
          const previous = this.tasks.find((item) => item.id === task.id)
          if (
            previous &&
            !terminal.includes(previous.status) &&
            terminal.includes(task.status)
          ) {
            completed = true
          }
          this.upsertTask(task)
        })
        if (completed) {
          this.loadTaskHistory()
          if (!this.batchMode) this.loadInventory()
        }
        if (this.tasks.some((task) => !terminal.includes(task.status))) {
          // WEB-024: re-check liveness just before re-scheduling so a request
          // that resolved after unmount cannot create a fresh timer.
          if (this._isDestroyed || this._isBeingDestroyed) return
          this.timer = setTimeout(() => this.loadTasks([]), 2000)
        }
      })
    },
    retryTask(task) {
      retryKernelTask({ id: task.id }).then((response) => {
        const retriedTask = response.data
        if (retriedTask && retriedTask.id) {
          this.upsertTask(retriedTask)
          this.rememberTasks([retriedTask.id])
          return this.loadTasks([retriedTask.id])
        }
        return this.loadTasks([task.id])
      })
    },
    enableMTLS() {
      MessageBox.prompt(
        this.$t('kernel.tlsServerNameRequired'),
        this.$t('kernel.probeMTLS'),
        {
          inputValue: this.currentServer.grpcTlsServerName || '',
          inputPattern: /^[A-Za-z0-9.-]{4,253}$/,
          inputErrorMessage: this.$t('kernel.tlsServerNameRequired')
        }
      ).then(({ value }) =>
        MessageBox.confirm(
          this.$t('kernel.mtlsNoFallback'),
          this.$t('confirm.warn'),
          {
            type: 'warning'
          }
        ).then(() =>
          probeKernelMTLS({
            nodeServerId: this.serverId,
            serverName: value
          }).then(() => {
            this.currentServer.grpcTlsMode = 'mtls'
            this.currentServer.grpcTlsServerName = value
            this.$message.success(this.$t('kernel.mtlsEnabled'))
            return this.loadInventory()
          })
        )
      )
    }
  }
}
</script>

<style scoped>
.section {
  margin-bottom: 20px;
}
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--hairline);
  color: var(--ink);
  font-weight: 700;
}
.inventory-warning {
  margin-bottom: 16px;
}
.hash {
  font-family: monospace;
  word-break: break-all;
}
.submit {
  margin-top: 20px;
}
</style>
