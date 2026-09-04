<template>
  <div class="dashboard-table-block">
    <div class="dashboard-filters">
      <liquid-select
        v-model="nodeServerId"
        clearable
        class="dashboard-server-select"
        :placeholder="$t('traffic.allServers')"
        @change="handleServerChange"
      >
        <option
          v-for="server in servers"
          :key="server.id"
          :label="server.name"
          :value="server.id"
        />
      </liquid-select>
      <traffic-period-filter
        v-model="filter"
        class="server-traffic-period-filter"
        aria-label="服务器流量周期"
        @change="handlePeriodChange"
      />
    </div>
    <liquid-table
      :data="rows"
      v-liquid-loading="loading"
      class="full-width-table"
    >
      <liquid-table-column
        prop="nodeServerName"
        :label="$t('table.nodeServerName')"
        min-width="170"
      />
      <liquid-table-column
        prop="upload"
        :label="$t('table.upload')"
        min-width="130"
        ><template slot-scope="s">{{
          getFlow(s.row.upload)
        }}</template></liquid-table-column
      >
      <liquid-table-column
        prop="download"
        :label="$t('table.download')"
        min-width="130"
        ><template slot-scope="s">{{
          getFlow(s.row.download)
        }}</template></liquid-table-column
      >
      <liquid-table-column
        prop="total"
        :label="$t('traffic.combined')"
        min-width="130"
        ><template slot-scope="s">{{
          getFlow(s.row.total)
        }}</template></liquid-table-column
      >
      <liquid-table-column
        :label="$t('table.actions')"
        width="112"
        align="center"
      >
        <template slot-scope="scope">
          <button
            class="cap small"
            type="button"
            @click="showDetails(scope.row)"
          >
            <app-icon name="view" />{{ $t('table.detail') }}
          </button>
        </template>
      </liquid-table-column>
    </liquid-table>
    <pagination
      v-if="total > query.pageSize"
      :total="total"
      :page.sync="query.pageNum"
      :limit.sync="query.pageSize"
      @pagination="fetchData"
    />

    <ui-dialog
      append-to-body
      :title="detailTitle"
      :visible.sync="detailDialogVisible"
      width="760px"
      :motion-key="
        detailServer ? 'server-traffic-' + detailServer.nodeServerId : ''
      "
      @close="clearDetails"
    >
      <liquid-table
        :data="detailRows"
        v-liquid-loading="detailLoading"
        class="full-width-table"
      >
        <liquid-table-column
          prop="username"
          :label="$t('dashboard.username')"
          min-width="160"
        />
        <liquid-table-column
          prop="upload"
          :label="$t('table.upload')"
          min-width="130"
          ><template slot-scope="s">{{
            getFlow(s.row.upload)
          }}</template></liquid-table-column
        >
        <liquid-table-column
          prop="download"
          :label="$t('table.download')"
          min-width="130"
          ><template slot-scope="s">{{
            getFlow(s.row.download)
          }}</template></liquid-table-column
        >
        <liquid-table-column
          prop="total"
          :label="$t('traffic.combined')"
          min-width="130"
          ><template slot-scope="s">{{
            getFlow(s.row.total)
          }}</template></liquid-table-column
        >
      </liquid-table>
      <pagination
        v-if="detailTotal > detailQuery.pageSize"
        :total="detailTotal"
        :page.sync="detailQuery.pageNum"
        :limit.sync="detailQuery.pageSize"
        @pagination="fetchDetails"
      />
    </ui-dialog>
  </div>
</template>

<script>
import { serverTrafficUsage, serverTrafficUserUsage } from '@/api/dashboard'
import { selectNodeServerList } from '@/api/node-server'
import { getFlow } from '@/utils/account'
import Pagination from '@/components/Pagination'
import TrafficPeriodFilter from './TrafficPeriodFilter'

export default {
  name: 'ServerTrafficTable',
  components: { Pagination, TrafficPeriodFilter },
  data() {
    return {
      rows: [],
      servers: [],
      total: 0,
      loading: false,
      requestSerial: 0,
      nodeServerId: undefined,
      filter: { period: 'total', date: '' },
      query: {
        pageNum: 1,
        pageSize: 20
      },
      detailDialogVisible: false,
      detailServer: null,
      detailRows: [],
      detailTotal: 0,
      detailLoading: false,
      detailRequestSerial: 0,
      detailQuery: {
        pageNum: 1,
        pageSize: 20
      }
    }
  },
  computed: {
    detailTitle() {
      if (!this.detailServer) return this.$t('traffic.serverUserUsage')
      return this.$t('traffic.serverUserUsageTitle', {
        name: this.detailServer.nodeServerName
      })
    }
  },
  created() {
    selectNodeServerList({}).then((response) => {
      this.servers = response.data
    })
    this.fetchData()
  },
  methods: {
    getFlow,
    handleServerChange() {
      this.query.pageNum = 1
      this.fetchData()
    },
    handlePeriodChange() {
      this.query.pageNum = 1
      this.fetchData()
      if (this.detailDialogVisible) {
        this.detailQuery.pageNum = 1
        this.fetchDetails()
      }
    },
    fetchData() {
      const requestSerial = ++this.requestSerial
      this.loading = true
      serverTrafficUsage({
        ...this.query,
        ...this.filter,
        nodeServerId: this.nodeServerId
      })
        .then((response) => {
          if (requestSerial !== this.requestSerial) return
          this.rows = response.data.rows
          this.total = response.data.total
        })
        .finally(() => {
          if (requestSerial === this.requestSerial) this.loading = false
        })
    },
    showDetails(server) {
      this.detailServer = server
      this.detailRows = []
      this.detailTotal = 0
      this.detailQuery.pageNum = 1
      this.detailDialogVisible = true
      this.fetchDetails()
    },
    fetchDetails() {
      if (!this.detailServer) return
      const requestSerial = ++this.detailRequestSerial
      this.detailLoading = true
      serverTrafficUserUsage({
        ...this.detailQuery,
        ...this.filter,
        nodeServerId: this.detailServer.nodeServerId
      })
        .then((response) => {
          if (requestSerial !== this.detailRequestSerial) return
          this.detailRows = response.data.rows
          this.detailTotal = response.data.total
        })
        .finally(() => {
          if (requestSerial === this.detailRequestSerial) {
            this.detailLoading = false
          }
        })
    },
    clearDetails() {
      this.detailServer = null
      this.detailRows = []
      this.detailTotal = 0
      this.detailLoading = false
      this.detailRequestSerial++
    }
  }
}
</script>

<style scoped>
.server-traffic-period-filter {
  flex: 1 0 100%;
  width: 100%;
}
</style>
