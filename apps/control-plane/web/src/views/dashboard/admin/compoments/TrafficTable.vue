<template>
  <div class="dashboard-table-block">
    <traffic-period-filter
      v-model="filter"
      aria-label="排行周期"
      @change="fetchData"
    />
    <liquid-table
      :data="list"
      class="full-width-table"
      v-liquid-loading="loading"
    >
      <liquid-table-column
        :label="$t('dashboard.ranking')"
        width="100"
        align="center"
        type="index"
      />
      <liquid-table-column
        prop="username"
        :label="$t('dashboard.username')"
        width="200"
        align="center"
      >
        <template slot-scope="scope">
          {{ scope.row.username }}
        </template>
      </liquid-table-column>
      <liquid-table-column
        prop="trafficUsed"
        :label="$t('dashboard.trafficUsed')"
        min-width="200"
        align="center"
      >
        <template slot-scope="scope">
          {{ getFlow(scope.row.trafficUsed) }}</template
        >
      </liquid-table-column>
    </liquid-table>
  </div>
</template>

<script>
import { trafficRank } from '@/api/dashboard'
import { getFlow } from '@/utils/account'
import TrafficPeriodFilter from './TrafficPeriodFilter'

export default {
  name: 'TrafficTable',
  components: { TrafficPeriodFilter },
  data() {
    return {
      list: null,
      filter: { period: 'total', date: '' },
      requestSerial: 0,
      loading: false
    }
  },
  created() {
    this.fetchData()
  },
  methods: {
    getFlow,
    fetchData() {
      const requestSerial = ++this.requestSerial
      this.loading = true
      trafficRank(this.filter)
        .then((response) => {
          if (requestSerial === this.requestSerial) {
            this.list = response.data
          }
        })
        .finally(() => {
          if (requestSerial === this.requestSerial) {
            this.loading = false
          }
        })
    }
  }
}
</script>
