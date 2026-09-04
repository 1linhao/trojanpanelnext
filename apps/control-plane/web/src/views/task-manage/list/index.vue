<template>
  <div class="prototype-page grid">
    <ui-panel motion-key="task-filters">
      <div class="toolbar">
        <div class="search-box">
          <app-icon name="search" /><input
            v-model="listQuery.accountUsername"
            placeholder="按账号搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <button class="cap primary small" type="button" @click="handleFilter">
          搜索任务
        </button>
      </div>
    </ui-panel>
    <ui-panel motion-key="task-list">
      <div class="tbl-wrap" v-liquid-loading="listLoading">
        <table class="tbl">
          <thead>
            <tr>
              <th>任务</th>
              <th>类型</th>
              <th>账号</th>
              <th>状态</th>
              <th>错误信息</th>
              <th>创建时间</th>
              <th class="table-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!listLoading && listError" class="tbl-empty">
              <td colspan="7">{{ listError }}</td>
            </tr>
            <tr v-else-if="!listLoading && !list.length" class="tbl-empty">
              <td colspan="7">暂无数据</td>
            </tr>
            <tr v-for="(row, index) in list" :key="row.id">
              <td class="primary-cell">
                <strong>{{ row.name }}</strong
                ><small>#{{ row.id }}</small>
              </td>
              <td>
                <span class="chip info">{{ typeComputed(row.type) }}</span>
              </td>
              <td class="muted">{{ row.accountUsername }}</td>
              <td>
                <span
                  class="chip"
                  :class="
                    row.status === 2 ? 'ok' : row.status === -1 ? 'bad' : 'warn'
                  "
                  ><span class="dot"></span
                  >{{ statusComputed(row.status) }}</span
                >
              </td>
              <td class="muted">{{ row.errMsg || '—' }}</td>
              <td class="muted">
                {{ timeStampToDate(row.createTime, false) }}
              </td>
              <td>
                <div class="row-actions">
                  <button
                    v-if="checkPermission(['sysadmin', 'admin'])"
                    class="cap small"
                    type="button"
                    :disabled="
                      row.status !== 2 || row.type === 3 || row.type === 4
                    "
                    @click="handleDownload(row, index)"
                  >
                    <app-icon name="download" />下载
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
  </div>
</template>

<script>
import Pagination from '@/components/Pagination'
import checkPermission from '@/utils/permission'
import { timeStampToDate } from '@/utils'
import { downloadFileTask, selectFileTaskPage } from '@/api/file-task'
import latestListRequest from '@/mixins/latest-list-request'

export default {
  mixins: [latestListRequest],
  name: 'TaskList',
  components: { Pagination },
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
        accountUsername: undefined
      },
      temp: {
        id: undefined,
        name: '',
        path: '',
        type: 1,
        status: 0,
        errMsg: '',
        accountUsername: '',
        createTime: new Date()
      }
    }
  },
  created() {
    this.getList()
  },
  computed: {
    typeComputed() {
      return function (type) {
        if (type === 1) {
          return this.$t('table.taskTypeAccountExport')
        } else if (type === 2) {
          return this.$t('table.taskTypeNodeServerExport')
        } else if (type === 3) {
          return this.$t('table.taskTypeAccountImport')
        } else if (type === 4) {
          return this.$t('table.taskTypeNodeServerImport')
        }
        return ''
      }
    },
    statusComputed() {
      return function (status) {
        if (status === -1) {
          return this.$t('table.taskFail')
        } else if (status === 0) {
          return this.$t('table.taskWait')
        } else if (status === 1) {
          return this.$t('table.taskDoing')
        } else if (status === 2) {
          return this.$t('table.taskSuccess')
        }
        return ''
      }
    }
  },
  filters: {
    statusTypeFilter(status) {
      if (status === -1) {
        return 'danger'
      } else if (status === 0) {
        return 'warning'
      } else if (status === 1) {
        return 'info'
      } else if (status === 2) {
        return 'success'
      }
      return ''
    }
  },
  methods: {
    checkPermission,
    timeStampToDate,
    getList() {
      const request = this.beginListRequest()
      return selectFileTaskPage(this.listQuery).then((response) => {
        if (!this.ownsListRequest(request)) return
        this.list = response.data.fileTasks
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
    handleDownload(row) {
      const tempData = Object.assign({}, row)
      downloadFileTask(tempData).then((res) => {
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
</style>
