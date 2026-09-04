<template>
  <div class="prototype-page grid">
    <ui-panel motion-key="email-filters">
      <div class="toolbar">
        <div class="search-box">
          <app-icon name="search" /><input
            v-model="listQuery.toEmail"
            placeholder="按收件邮箱搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <liquid-select
          v-model="listQuery.state"
          clearable
          placeholder="发送状态"
          class="prototype-select"
          ><option
            v-for="item in states"
            :key="item.value"
            :label="item.label"
            :value="item.value" /></liquid-select
        ><button class="cap primary small" type="button" @click="handleFilter">
          搜索记录
        </button>
      </div>
    </ui-panel>
    <ui-panel motion-key="email-list">
      <div class="tbl-wrap" v-liquid-loading="listLoading">
        <table class="tbl">
          <thead>
            <tr>
              <th>收件人</th>
              <th>主题</th>
              <th>内容</th>
              <th>状态</th>
              <th>创建时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!listLoading && listError" class="tbl-empty">
              <td colspan="5">{{ listError }}</td>
            </tr>
            <tr v-else-if="!listLoading && !list.length" class="tbl-empty">
              <td colspan="5">暂无数据</td>
            </tr>
            <tr v-for="row in list" :key="row.id">
              <td class="primary-cell">
                <strong>{{ row.toEmail }}</strong
                ><small>#{{ row.id }}</small>
              </td>
              <td>{{ row.subject }}</td>
              <td class="muted email-content">{{ row.content }}</td>
              <td>
                <span
                  class="chip"
                  :class="
                    row.state === 1 ? 'ok' : row.state === -1 ? 'bad' : 'warn'
                  "
                  ><span class="dot"></span
                  >{{ stateDescFilter(row.state) }}</span
                >
              </td>
              <td class="muted">
                {{ timeStampToDate(row.createTime, false) }}
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
  </div>
</template>

<script>
import Pagination from '@/components/Pagination'
import { timeStampToDate } from '@/utils'
import { selectEmailRecordPage } from '@/api/email-record'
import latestListRequest from '@/mixins/latest-list-request'

export default {
  mixins: [latestListRequest],
  name: 'EmailRecordPage',
  components: { Pagination },

  filters: {
    stateFilter(state) {
      let stateMap = new Map()
      stateMap.set(-1, 'danger')
      stateMap.set(0, 'warn')
      stateMap.set(1, 'success')
      return stateMap.get(state)
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
        toEmail: undefined,
        state: undefined
      },
      temp: {
        id: undefined,
        toEmail: '',
        subject: '',
        content: '',
        state: 0,
        createTime: new Date()
      },
      states: [
        { value: -1, label: this.$t('table.sendFail') },
        { value: 0, label: this.$t('table.sendWait') },
        { value: 1, label: this.$t('table.sendSuccess') }
      ],
      stateDesc: [
        { value: -1, desc: this.$t('table.sendFail') },
        { value: 0, desc: this.$t('table.sendWait') },
        { value: 1, desc: this.$t('table.sendSuccess') }
      ]
    }
  },
  created() {
    this.getList()
  },
  methods: {
    timeStampToDate,
    getList() {
      const request = this.beginListRequest()
      return selectEmailRecordPage(this.listQuery).then((response) => {
        if (!this.ownsListRequest(request)) return
        this.list = response.data.emailRecords
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
    stateDescFilter(state) {
      let stateDesc = this.stateDesc.find((item) => item.value === state)
      if (stateDesc && stateDesc.desc) {
        return stateDesc.desc
      } else {
        return ''
      }
    }
  }
}
</script>

<style scoped>
.liquid-button {
  margin-left: 10px;
}
</style>
