<template>
  <div class="prototype-page grid">
    <ui-panel motion-key="node-filters">
      <div class="toolbar">
        <div class="search-box">
          <app-icon name="search" /><input
            v-model="listQuery.name"
            placeholder="按节点名搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <liquid-select
          v-model="listQuery.nodeServerId"
          clearable
          placeholder="全部服务器"
          class="prototype-select"
          @change="handleFilter"
        >
          <option
            v-for="item in nodeServers"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </liquid-select>
        <div class="spacer"></div>
        <button
          class="cap small"
          type="button"
          @click="exportDialogVisible = true"
        >
          <app-icon name="export" />订阅导出
        </button>
        <button
          v-if="checkPermission(['sysadmin', 'admin'])"
          class="cap primary small"
          type="button"
          @click="handleCreate"
        >
          <app-icon name="plus" />新建节点
        </button>
      </div>
    </ui-panel>

    <div class="node-grid" v-liquid-loading="listLoading">
      <p v-if="!listLoading && listError" class="node-grid__empty">{{ listError }}</p>
      <p v-else-if="!listLoading && !list.length" class="node-grid__empty">暂无数据</p>
      <template v-for="(row, index) in list">
        <ui-panel
          :key="'node-' + row.id"
          variant="metric"
          class="node-card"
          motion-role="shared"
          :motion-key="detailId === row.id ? '' : 'node-' + row.id"
        >
          <div class="node-top">
            <span
              class="proto-badge tone-blue"
              :title="nodeTypeFind(nodeTypes, row.nodeTypeId)"
              >{{
              nodeTypeFind(nodeTypes, row.nodeTypeId)
            }}</span>
            <div class="grow">
              <h3>{{ row.name }}</h3>
              <span class="muted"
                >{{ nodeServerFind(nodeServers, row.nodeServerId) }} ·
                {{ row.domain }}:{{ row.port }}</span
              >
            </div>
          </div>
          <div class="node-meta">
            <span class="chip plain">{{
              row.xrayProtocol || nodeTypeFind(nodeTypes, row.nodeTypeId)
            }}</span>
            <span class="chip info">优先级 {{ row.priority }}</span>
            <span
              v-if="checkPermission(['sysadmin', 'admin'])"
              class="chip"
              :class="row.status === 1 ? 'ok' : 'bad'"
              ><span class="dot"></span>{{ statusComputed(row.status) }}</span
            >
          </div>
          <div v-if="row.clients && row.clients.length" class="node-meta">
            <span
              v-for="client in row.clients"
              :key="client"
              class="chip violet"
              >{{ client }}</span
            >
          </div>
          <div class="node-foot">
            <span
              v-if="!row.serverTraffic || row.serverTraffic.period === 'none'"
              class="latency"
              >服务器流量不限额</span
            >
            <span v-else-if="row.serverTraffic.reached" class="latency bad-text"
              >服务器流量已达限额</span
            >
            <span v-else class="latency"
              >剩余
              {{
                row.serverTraffic.limitMode === 'separate'
                  ? '↑ ' +
                    quotaFlow(
                      row.serverTraffic.uploadLimit,
                      row.serverTraffic.uploadRemaining
                    ) +
                    ' / ↓ ' +
                    quotaFlow(
                      row.serverTraffic.downloadLimit,
                      row.serverTraffic.downloadRemaining
                    )
                  : getFlow(row.serverTraffic.totalRemaining)
              }}</span
            >
            <button
              class="cap small"
              type="button"
              @click="handlePrototypeDetail(row)"
            >
              <app-icon name="view" />{{ detailId === row.id ? '收起' : '详情' }}
            </button>
            <template v-if="checkPermission(['sysadmin', 'admin'])">
              <button
                class="icon-btn"
                type="button"
                title="编辑"
                @click="handleUpdate(row)"
              >
                <app-icon name="edit" />
              </button>
              <button
                class="icon-btn danger"
                type="button"
                title="删除"
                @click="handleDelete(row, index)"
              >
                <app-icon name="delete" />
              </button>
            </template>
          </div>
        </ui-panel>
        <ui-sheet
          v-if="detailId === row.id"
          :key="'detail-' + row.id"
          class="node-detail"
          motion-role="shared"
          :motion-key="'node-' + row.id"
        >
          <div class="card-head">
            <div>
              <span class="kicker">Connection</span>
              <h2>{{ row.name }} · 连接参数</h2>
            </div>
                  <button class="icon-btn" type="button" aria-label="关闭节点详情" @click="detailId = 0">
              <app-icon name="close" />
            </button>
          </div>
          <div class="kv-grid">
            <div class="kv">
              <span>域名</span><b class="mono">{{ nodeDetail.domain }}</b>
            </div>
            <div class="kv">
              <span>端口</span><b class="mono">{{ nodeDetail.port }}</b>
            </div>
            <div class="kv">
              <span>连接密码</span
              ><b class="mono">{{
                nodeDetail.password || nodeDetail.pass || '—'
              }}</b>
            </div>
            <div class="kv">
              <span>协议</span
              ><b
                >{{ nodeTypeFind(nodeTypes, row.nodeTypeId) }} ·
                {{ row.xrayProtocol || '默认' }}</b
              >
            </div>
            <div v-if="nodeDetail.realityPbk" class="kv">
              <span>Reality 公钥</span
              ><b class="mono">{{ nodeDetail.realityPbk }}</b>
            </div>
            <div v-if="nodeDetail.uuid" class="kv">
              <span>UUID</span><b class="mono">{{ nodeDetail.uuid }}</b>
            </div>
          </div>
        </ui-sheet>
      </template>
    </div>
    <Pagination
      v-if="total > 0"
      :total="total"
      :page.sync="listQuery.pageNum"
      :limit.sync="listQuery.pageSize"
      @pagination="getList"
    />
    <NodeForm
      ref="nodeForm"
      :dialog-form-visible-props.sync="dialogFormVisible"
      :node-props="temp"
      :dialog-status-props="dialogStatus"
      :node-servers-props="nodeServers"
      :node-types-props="nodeTypes"
      :get-list-props="getList"
    />
    <ExportNodeDialog :dialog-visible-props.sync="exportDialogVisible" />
  </div>
</template>

<script>
import Pagination from '@/components/Pagination'
import ExportNodeDialog from '@/views/node/list/components/ExportNodeDialog'
import { MessageBox } from '@/utils/liquid-feedback'
import {
  deleteNodeById,
  nodeDefault,
  selectNodeById,
  selectNodeInfo,
  selectNodePage
} from '@/api/node'
import { selectNodeTypeList } from '@/api/node-type'
import {
  handleNodeDetail,
  handleNodeUpdate,
  nodeServerFind,
  nodeTypeFind
} from '@/utils/node'
import checkPermission from '@/utils/permission'
import { timeStampToDate } from '@/utils'
import { getFlow } from '@/utils/account'
import { selectNodeServerList } from '@/api/node-server'
import NodeForm from '@/views/node/list/components/NodeForm'
import latestListRequest from '@/mixins/latest-list-request'

export default {
  name: 'NodeListPage',
  components: {
    NodeForm,
    ExportNodeDialog,
    Pagination
  },
  mixins: [latestListRequest],
  filters: {
    statusTypeFilter(status) {
      return status > 0 ? 'success' : 'danger'
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
        name: undefined,
        nodeServerId: undefined
      },
      temp: {
        id: undefined,
        nodeServerId: undefined,
        nodeSubId: undefined,
        nodeTypeId: 1,
        name: '',
        domain: '',
        port: 443,
        priority: 100,
        clients: ['sing-box', 'clash-meta', 'v2ray', 'shadowrocket'],

        xrayProtocol: 'vless',
        xrayFlow: '',
        xraySSMethod: 'aes-256-gcm',
        xrayUotEnable: 0,
        xrayUotVersion: 2,
        xrayXudpEnable: 0,
        xrayMuxEnable: 0,
        realityPbk: '',
        xraySettings: '',
        xraySettingsEntity: {
          fallbacks: [
            {
              name: '',
              alpn: '',
              path: undefined,
              dest: '80',
              xver: 0
            }
          ],
          network: 'tcp',
          accounts: [{ user: '', pass: '' }],
          udp: true
        },
        xrayStreamSettings: '',
        xrayStreamSettingsEntity: {
          network: 'tcp',
          security: 'none',
          tlsSettings: {
            serverName: '',
            alpn: ['h2', 'http/1.1'],
            allowInsecure: false,
            fingerprint: 'chrome'
          },
          realitySettings: {
            dest: '',
            xver: 0,
            serverNames: [],
            fingerprint: 'chrome',
            privateKey: '',
            shortIds: [],
            spiderX: ''
          },
          wsSettings: {
            path: '/trojan-panel-websocket-path',
            headers: {
              Host: ''
            }
          }
        },
        xrayTag: 'user',
        xraySniffing: '',
        xrayAllocate: '',

        trojanGoSni: '',
        trojanGoMuxEnable: 1,
        trojanGoWebsocketEnable: 0,
        trojanGoWebsocketPath: '/trojan-panel-websocket-path',
        trojanGoWebsocketHost: '',
        trojanGoSsEnable: 0,
        trojanGoSsMethod: 'AES-128-GCM',
        trojanGoSsPassword: '',

        hysteriaProtocol: 'udp',
        hysteriaObfs: '',
        hysteriaUpMbps: 100,
        hysteriaDownMbps: 100,
        hysteriaServerName: '',
        hysteriaInsecure: 0,
        hysteriaFastOpen: 0,

        hysteria2ObfsPassword: '',
        hysteria2UpMbps: 100,
        hysteria2DownMbps: 100,
        hysteria2ServerName: '',
        hysteria2PortHopping: '',
        hysteria2HopInterval: undefined,
        hysteria2Insecure: 0,
        naiveUotEnable: 0,
        naiveUotVersion: 2,
        createTime: new Date()
      },
      nodeDetail: {
        id: undefined,
        nodeServerId: undefined,
        nodeSubId: undefined,
        nodeTypeId: 1,
        name: '',
        domain: '',
        port: 443,
        priority: 100,
        clients: ['sing-box', 'clash-meta', 'v2ray', 'shadowrocket'],

        password: '',
        uuid: '',
        alterId: 0,

        xrayProtocol: 'vless',
        xrayFlow: '',
        xraySSMethod: 'aes-256-gcm',
        xrayUotEnable: 0,
        xrayUotVersion: 2,
        xrayXudpEnable: 0,
        xrayMuxEnable: 0,
        realityPbk: '',
        xraySettings: '',
        xraySettingsEntity: {
          fallbacks: [
            {
              name: '',
              alpn: '',
              path: undefined,
              dest: '80',
              xver: 0
            }
          ],
          network: 'tcp',
          accounts: [{ user: '', pass: '' }],
          udp: true
        },
        xrayStreamSettings: '',
        xrayStreamSettingsEntity: {
          network: 'tcp',
          security: 'none',
          tlsSettings: {
            serverName: '',
            alpn: ['h2', 'http/1.1'],
            allowInsecure: false,
            fingerprint: 'chrome'
          },
          realitySettings: {
            dest: '',
            xver: 0,
            serverNames: [],
            fingerprint: 'chrome',
            privateKey: '',
            shortIds: [],
            spiderX: ''
          },
          wsSettings: {
            path: '/trojan-panel-websocket-path',
            headers: {
              Host: ''
            }
          }
        },
        xrayTag: 'user',
        xraySniffing: '',
        xrayAllocate: '',

        trojanGoSni: '',
        trojanGoMuxEnable: 1,
        trojanGoWebsocketEnable: 0,
        trojanGoWebsocketPath: '/trojan-panel-websocket-path',
        trojanGoWebsocketHost: '',
        trojanGoSsEnable: 0,
        trojanGoSsMethod: 'AES-128-GCM',
        trojanGoSsPassword: '',

        hysteriaProtocol: 'udp',
        hysteriaObfs: '',
        hysteriaUpMbps: 100,
        hysteriaDownMbps: 100,
        hysteriaServerName: '',
        hysteriaInsecure: 0,
        hysteriaFastOpen: 0,

        hysteria2ObfsPassword: '',
        hysteria2UpMbps: 100,
        hysteria2DownMbps: 100,
        hysteria2ServerName: '',
        hysteria2PortHopping: '',
        hysteria2HopInterval: undefined,
        hysteria2Insecure: 0,

        naiveProxyUsername: '',
        naiveUotEnable: 0,
        naiveUotVersion: 2,
        createTime: new Date()
      },
      dialogStatus: '',
      dialogFormVisible: false,
      dialogInfoVisible: false,
      exportDialogVisible: false,
      detailId: 0,
      nodeTypes: [],
      nodeServers: [],
      textMap: {
        update: this.$t('table.edit'),
        create: this.$t('table.add')
      }
    }
  },
  created() {
    this.setNodeTypes()
    this.setNodeServers()
    this.getList()
  },
  methods: {
    handlePrototypeDetail(row) {
      if (this.detailId === row.id) {
        this.detailId = 0
        return
      }
      this.nodeDetail = Object.assign({}, row)
      selectNodeInfo({ id: row.id }).then((response) => {
        try {
          this.nodeDetail = handleNodeDetail(this.nodeDetail, response.data)
        } catch (error) {
          this.nodeDetail = Object.assign({}, this.nodeDetail, response.data)
        }
        this.detailId = row.id
      })
    },
    getFlow,
    quotaFlow(limit, remaining) {
      return limit > 0 ? getFlow(remaining) : this.$t('traffic.unlimited')
    },
    timeStampToDate,
    checkPermission,
    nodeServerFind,
    nodeTypeFind,
    setNodeTypes() {
      selectNodeTypeList().then((response) => {
        const { data } = response
        this.nodeTypes = data
      })
    },
    setNodeServers() {
      selectNodeServerList().then((response) => {
        const { data } = response
        this.nodeServers = data
      })
    },
    resetTemp() {
      this.temp = {
        id: undefined,
        nodeServerId: undefined,
        nodeSubId: undefined,
        nodeTypeId: 1,
        name: '',
        domain: '',
        port: 443,
        priority: 100,
        clients: ['sing-box', 'clash-meta', 'v2ray', 'shadowrocket'],

        xrayProtocol: 'vless',
        xrayFlow: '',
        xraySSMethod: 'aes-256-gcm',
        xrayUotEnable: 0,
        xrayUotVersion: 2,
        xrayXudpEnable: 0,
        xrayMuxEnable: 0,
        realityPbk: '',
        xraySettings: '',
        xraySettingsEntity: {
          fallbacks: [
            {
              name: '',
              alpn: '',
              path: undefined,
              dest: '80',
              xver: 0
            }
          ],
          network: 'tcp',
          accounts: [{ user: '', pass: '' }],
          udp: true
        },
        xrayStreamSettings: '',
        xrayStreamSettingsEntity: {
          network: 'tcp',
          security: 'none',
          tlsSettings: {
            serverName: '',
            alpn: ['h2', 'http/1.1'],
            allowInsecure: false,
            fingerprint: 'chrome'
          },
          realitySettings: {
            dest: '',
            xver: 0,
            serverNames: [],
            fingerprint: 'chrome',
            privateKey: '',
            shortIds: [],
            spiderX: ''
          },
          wsSettings: {
            path: '/trojan-panel-websocket-path',
            headers: {
              Host: ''
            }
          }
        },
        xrayTag: 'user',
        xraySniffing: '',
        xrayAllocate: '',

        trojanGoSni: '',
        trojanGoMuxEnable: 1,
        trojanGoWebsocketEnable: 0,
        trojanGoWebsocketPath: '/trojan-panel-websocket-path',
        trojanGoWebsocketHost: '',
        trojanGoSsEnable: 0,
        trojanGoSsMethod: 'AES-128-GCM',
        trojanGoSsPassword: '',

        hysteriaProtocol: 'udp',
        hysteriaObfs: '',
        hysteriaUpMbps: 100,
        hysteriaDownMbps: 100,
        hysteriaServerName: '',
        hysteriaInsecure: 0,
        hysteriaFastOpen: 0,

        hysteria2ObfsPassword: '',
        hysteria2UpMbps: 100,
        hysteria2DownMbps: 100,
        hysteria2ServerName: '',
        hysteria2PortHopping: '',
        hysteria2HopInterval: undefined,
        hysteria2Insecure: 0,
        naiveUotEnable: 0,
        naiveUotVersion: 2,
        createTime: new Date()
      }
    },
    getList() {
      const request = this.beginListRequest()
      return selectNodePage(this.listQuery).then((response) => {
        if (!this.ownsListRequest(request)) return
        this.list = response.data.nodes
        this.total = response.data.total
      }).catch(() => {
        if (!this.ownsListRequest(request)) return
        this.list = []
        this.total = 0
        this.listError = '请求失败，请重试'
      }).finally(() => this.finishListRequest(request))
    },
    handleFilter() {
      this.listQuery.pageNum = 1
      this.getList()
    },
    handleCreate() {
      this.resetTemp()
      // 设置节点部分属性的默认值
      nodeDefault().then((response) => {
        this.temp.realityPbk = response.data.publicKey
        this.temp.xrayStreamSettingsEntity.realitySettings.privateKey =
          response.data.privateKey
        this.temp.xrayStreamSettingsEntity.realitySettings.shortIds = [
          response.data.shortId
        ]
        this.temp.xrayStreamSettingsEntity.realitySettings.spiderX =
          response.data.spiderX
      })
      this.dialogStatus = 'create'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['nodeForm'].$refs['dataForm'].clearValidate()
      })
    },
    handleUpdate(row) {
      this.temp = Object.assign(this.temp, row)
      selectNodeById({ id: row.id }).then((response) => {
        this.temp = handleNodeUpdate(this.temp, response.data)
        this.dialogStatus = 'update'
        this.dialogFormVisible = true
        this.$nextTick(() => {
          this.$refs['nodeForm'].$refs['dataForm'].clearValidate()
        })
      })
    },
    handleDetail(row) {
      this.nodeDetail = Object.assign(this.temp, row)
      selectNodeInfo({ id: row.id }).then((response) => {
        this.nodeDetail = handleNodeDetail(this.nodeDetail, response.data)
        this.dialogInfoVisible = true
      })
    },
    handleDelete(row, index) {
      MessageBox.confirm(
        this.$t('confirm.deleteNode').toString(),
        this.$t('confirm.warn').toString(),
        {
          confirmButtonText: this.$t('confirm.yes').toString(),
          cancelButtonText: this.$t('confirm.cancel').toString(),
          type: 'warning'
        }
      ).then(() => {
        const tempData = Object.assign({}, row)
        deleteNodeById(tempData).then(() => {
          this.list.splice(index, 1)
          this.$notify({
            title: 'Success',
            message: this.$t('confirm.deleteSuccess').toString(),
            type: 'success',
            duration: 2000
          })
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
</style>
