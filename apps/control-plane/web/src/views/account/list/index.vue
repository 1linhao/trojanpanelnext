<template>
  <div class="prototype-page grid">
    <ui-panel motion-key="account-filters">
      <div class="toolbar">
        <div class="search-box">
          <app-icon name="search" />
          <input
            v-model="listQuery.username"
            placeholder="按用户名搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <liquid-select
          v-model="listQuery.deleted"
          clearable
          placeholder="账号状态"
          class="prototype-select"
          @change="handleStatusFilter"
        >
          <option
            v-for="item in deletedList"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </liquid-select>
        <div class="spacer"></div>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="handleCreateBatch"
        >
          <app-icon name="user-plus" />批量创建
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
          @click="exportAccountUnused"
        >
          <app-icon name="export" />导出未使用
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          :disabled="resettingAllTraffic"
          :title="$t('table.resetAllAccountTrafficLabel')"
          :aria-label="$t('table.resetAllAccountTrafficLabel')"
          @click="handleResetAllAccountTraffic"
        >
          <app-icon name="refresh" aria-hidden="true" />{{ $t('table.resetAllAccountTraffic') }}
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap primary small"
          type="button"
          @click="handleCreate"
        >
          <app-icon name="plus" />新建账号
        </button>
      </div>
    </ui-panel>

    <ui-panel motion-key="account-list">
      <div
        ref="accountTable"
        v-liquid-loading="listLoading"
        class="tbl-wrap"
        role="region"
        :aria-label="$t('route.accountList')"
        tabindex="0"
      >
        <table class="tbl">
          <thead>
            <tr>
              <th>账号</th>
              <th v-if="canViewRemarks">{{ $t('table.accountRemark') }}</th>
              <th>角色</th>
              <th>流量（已用 / 配额）</th>
              <th class="account-expiry-column">到期时间</th>
              <th>最近登录</th>
              <th>状态</th>
              <th scope="col" class="sticky-row-actions">{{ $t('table.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!listLoading && listError" class="tbl-empty">
              <td :colspan="tableColumnCount">{{ listError }}</td>
            </tr>
            <tr v-else-if="!listLoading && !list.length" class="tbl-empty">
              <td :colspan="tableColumnCount">暂无数据</td>
            </tr>
            <tr v-for="row in list" :key="row.id">
              <td class="primary-cell">
                <strong>{{ row.username }}</strong
                ><small>{{ row.email || '未绑定邮箱' }}</small>
              </td>
              <td v-if="canViewRemarks" class="account-remark-cell">{{ row.remark || '—' }}</td>
              <td>
                <span
                  class="chip"
                  :class="row.roleId === 1 ? 'violet' : 'info'"
                  >{{ roleFilter(row.roleId) }}</span
                >
              </td>
              <td class="account-actions-cell">
                <div class="traffic-label">
                  <span class="muted num">{{
                    getFlow(Number(row.upload || 0) + Number(row.download || 0))
                  }}</span
                  ><span class="faint num">{{
                    row.quota < 0 ? '不限' : getFlow(row.quota)
                  }}</span>
                </div>
                <div
                  class="meter"
                  :class="{
                    warn: accountUsagePct(row) > 70,
                    bad: accountUsagePct(row) > 92
                  }"
                >
                  <i :style="{ width: accountUsagePct(row) + '%' }"></i>
                </div>
              </td>
              <td class="muted account-expiry-column">
                <span v-if="row.lastLoginTime === 0">首次登录后计算</span>
                <time v-else class="account-expiry-value">
                  <span>{{ timePart(row.expireTime, 0) }}</span>
                  <span>{{ timePart(row.expireTime, 1) }}</span>
                  <span v-if="row.expireTime <= accountNow" class="bad-text">{{ $t('accountStatus.expired') }}</span>
                </time>
              </td>
              <td class="muted">
                {{
                  row.lastLoginTime
                    ? timeStampToDate(row.lastLoginTime, false)
                    : '从未登录'
                }}
              </td>
              <td>
                <span class="chip" :class="row.deleted === 0 ? 'ok' : 'bad'"
                  ><span class="dot"></span
                  >{{ row.deleted === 0 ? $t('table.enable') : $t('table.disable') }}</span
                >
              </td>
              <td class="sticky-row-actions">
                <liquid-action-menu
                  v-if="checkPermission(['sysadmin']) || (checkPermission(['admin']) && row.lastLoginTime !== 0)"
                  :label="$t('table.actionsFor', { name: row.username })"
                >
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    :title="$t('table.edit')"
                    @click="handleUpdate(row)"
                  >
                    <app-icon name="edit" aria-hidden="true" />
                    {{ $t('table.edit') }}
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    :title="$t('accountNodeAccess.action')"
                    @click="nodeAccessAccount = row"
                  >
                    <app-icon name="node" aria-hidden="true" />
                    {{ $t('accountNodeAccess.action') }}
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    :title="$t('table.resetAccountTraffic')"
                    @click="handleReset(row)"
                  >
                    <app-icon name="reset-traffic" aria-hidden="true" />
                    {{ $t('table.resetAccountTraffic') }}
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    :title="$t('table.resetAccountLoginLimit')"
                    :disabled="resettingLoginLimitId !== 0"
                    @click="handleResetLoginLimit(row)"
                  >
                    <app-icon name="reset-login" aria-hidden="true" />
                    {{ $t('table.resetAccountLoginLimit') }}
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin', 'admin']) && row.lastLoginTime !== 0"
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    :title="$t('table.clashSubscribeForSb')"
                    @click="handleClashSubscribeForSb(row)"
                  >
                    <app-icon name="document-copy" aria-hidden="true" />
                    {{ $t('table.clashSubscribeForSb') }}
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    class="danger"
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    :title="$t('table.delete')"
                    @click="handleDelete(row)"
                  >
                    <app-icon name="delete" aria-hidden="true" />
                    {{ $t('table.delete') }}
                  </button>
                </liquid-action-menu>
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
        @pagination="handlePagination"
      />
    </ui-panel>
    <ui-dialog
      append-to-body
      :title="textMap[dialogStatus]"
      :visible.sync="dialogFormVisible"
    >
      <liquid-form
        ref="dataForm"
        :rules="dialogStatus === 'create' ? createRules : updateRules"
        :model="temp"
        label-position="left"
      >
        <liquid-form-item
          v-if="dialogStatus === 'create'"
          :label="$t('table.username')"
          prop="username"
        >
          <liquid-input
            v-model="temp.username"
            :placeholder="$t('table.username')"
            clearable
          />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.pass')" prop="pass">
          <liquid-input
            v-model="temp.pass"
            type="password"
            :placeholder="$t('table.pass')"
            clearable
          />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.editQuota')" prop="quota">
          <liquid-number-input
            v-model.number="temp.quota"
            controls-position="right"
            type="number"
          />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.email')" prop="email">
          <liquid-input
            v-model="temp.email"
            :placeholder="$t('table.email')"
            clearable
          />
        </liquid-form-item>
        <liquid-form-item
          v-if="dialogStatus === 'update' && canViewRemarks"
          :label="$t('table.accountRemark')"
          prop="remark"
        >
          <liquid-input v-model="temp.remark" type="textarea" :rows="3" clearable />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.status')" prop="deleted">
          <liquid-switch
            v-model="temp.deleted"
            :active-text="$t('table.enable')"
            :inactive-text="$t('table.disable')"
            :active-value="0"
            :inactive-value="1"
          >
          </liquid-switch>
          <p v-if="temp.deleted === 1" class="ui-supporting-text">{{ $t('accountStatus.disabledHint') }}</p>
        </liquid-form-item>
        <liquid-form-item :label="$t('table.expireTime')" prop="expireTime">
          <liquid-date-picker
            v-model="temp.expireTime"
            type="datetime"
            value-format="timestamp"
            :placeholder="$t('table.expireTime')"
          />
          <p v-if="dialogStatus === 'update' && temp.quota === 0" class="ui-supporting-text">{{ $t('accountStatus.renewalHint') }}</p>
        </liquid-form-item>
      </liquid-form>
      <div slot="footer" class="dialog-footer">
        <liquid-button @click="dialogFormVisible = false"
          >{{ $t('table.cancel') }}
        </liquid-button>
        <liquid-button
          type="primary"
          @click="dialogStatus === 'create' ? createData() : updateData()"
        >
          {{ $t('table.confirm') }}
        </liquid-button>
      </div>
    </ui-dialog>
    <AccountNodeAccess
      v-if="nodeAccessAccount && canViewRemarks"
      :account="nodeAccessAccount"
      @close="nodeAccessAccount = null"
    />
    <import-tip
      ref="importTip"
      :dialog-form-visible.sync="importVisible"
      :label="$t('table.coverByAccountName')"
      :import-data="importData"
      :download-template="downloadTemplate"
      :retry="importFailures.length > 0"
    >
      <template #retry>
        <div v-if="importFailures.length" class="account-import-retry">
          <p role="status">{{ $t('accountImport.summary', importResult) }}</p>
          <label v-for="row in importFailures" :key="row.number">
            {{ $t('accountImport.row', { row: row.number, reason: $t(`accountImport.${row.reason}`) }) }}
            <liquid-input v-model="row.source" type="textarea" spellcheck="false" />
          </label>
        </div>
      </template>
    </import-tip>
    <BatchOperation
      ref="batchOperationForm"
      :dialog-form-visible-props.sync="batchOperationDialogFormVisible"
      :get-list-props="getList"
    />
  </div>
</template>

<script>
import {
  createAccount,
  deleteAccountById,
  downloadAccountImportTemplate,
  exportAccount,
  exportAccountUnused,
  exportSubscribe,
  importAccount,
  resetAccountDownloadAndUpload,
  resetAllAccountDownloadAndUpload,
  resetAccountLoginLimit,
  selectAccountPage,
  updateAccountById
} from '@/api/account'
import Pagination from '@/components/Pagination'
import LiquidActionMenu from '@/components/LiquidActionMenu'
import ImportTip from '@/components/ImportTip'
import latestListRequest from '@/mixins/latest-list-request'

import { Message, MessageBox } from '@/utils/liquid-feedback'
import { timeStampToDate } from '@/utils'
import { byteToMb, getFlow } from '@/utils/account'
import { selectRoleList } from '@/api/role'
import checkPermission from '@/utils/permission'
import { setting } from '@/api/system'
import accountExpiryClock from '@/mixins/account-expiry-clock'
import BatchOperation from '@/views/account/list/compoments/BatchOperation'
import AccountNodeAccess from '@/views/account/list/compoments/AccountNodeAccess'
import copy from 'copy-to-clipboard'
import accountImportRows from './account-import-rows'

export default {
  name: 'AccountListPage',
  filters: {
    deletedFilter(deleted) {
      const deletedMap = {
        0: 'success',
        1: 'danger'
      }
      return deletedMap[deleted]
    }
  },
  components: { AccountNodeAccess, BatchOperation, Pagination, ImportTip, LiquidActionMenu },
  mixins: [latestListRequest, accountExpiryClock],
  data() {
    const validateUsername = (rule, value, callback) => {
      if (this.temp.username.trim().indexOf('admin') >= 0) {
        callback(new Error(this.$t('valid.usernameNotExistAdmin')))
      } else {
        callback()
      }
    }
    const validateAccountRemark = (rule, value, callback) => {
      if (!checkPermission(['sysadmin']) || value === undefined || value === null) { callback(); return }
      callback(typeof value === 'string' && Array.from(value).length <= 500
        ? undefined : new Error(this.$t('valid.accountRemarkRange')))
    }
    return {
      tableKey: 0,
      listLoading: true,
      listError: '',
      list: null,
      total: 0,
      resettingLoginLimitId: 0,
      originalQuota: null,
      originalExpiry: null,
      resettingAllTraffic: false,
      orderFieldArr: ['role_id', 'create_time'],
      listQuery: {
        pageNum: 1,
        pageSize: 20,
        username: undefined,
        deleted: undefined,
        orderFields: 'role_id,create_time',
        orderBy: 'desc',
        lastLoginTime: undefined
      },
      temp: {
        id: undefined,
        quota: 0,
        download: 0,
        upload: 0,
        username: undefined,
        pass: undefined,
        email: undefined,
        remark: undefined,
        roleId: 3,
        deleted: 0,
        lastLoginTime: 0,
        expireTime: new Date().getTime(),
        createTime: new Date()
      },
      dialogFormVisible: false,
      batchOperationDialogFormVisible: false,
      textMap: {
        update: this.$t('table.edit'),
        create: this.$t('table.add')
      },
      importVisible: false,
      importFailures: [],
      importResult: null,
      deletedList: [
        { value: 0, label: this.$t('table.enable') },
        { value: 1, label: this.$t('table.disable') }
      ],
      lastLoginTimeList: [
        { value: 0, label: this.$t('table.no') },
        { value: 1, label: this.$t('table.yes') }
      ],
      orderFieldList: [
        { value: 'quota', label: this.$t('table.quota') },
        { value: 'role_id', label: this.$t('table.role') },
        { value: 'last_login_time', label: this.$t('table.lastLoginTime') },
        { value: 'expire_time', label: this.$t('table.expireTime') },
        { value: 'deleted', label: this.$t('table.deleted') },
        { value: 'create_time', label: this.$t('table.createTime') }
      ],
      orderByList: [
        { value: 'desc', label: this.$t('table.desc') },
        { value: 'asc', label: this.$t('table.asc') }
      ],
      createRules: {
        username: [
          {
            required: true,
            message: this.$t('valid.username'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.usernameRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.usernameElement'),
            trigger: ['change', 'blur']
          },
          {
            validator: validateUsername,
            trigger: ['change', 'blur']
          }
        ],
        pass: [
          {
            required: true,
            message: this.$t('valid.pass'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.passRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.passElement'),
            trigger: ['change', 'blur']
          }
        ],
        quota: [
          {
            required: true,
            message: this.$t('valid.quota'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: -1,
            max: 1024000,
            message: this.$t('valid.quotaRange'),
            trigger: ['change', 'blur']
          }
        ],
        email: [
          {
            min: 4,
            max: 64,
            message: this.$t('valid.emailRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^([A-Za-z0-9_.-])+@(163.com|126.com|qq.com|gmail.com)$/,
            message: this.$t('valid.emailElement'),
            trigger: ['change', 'blur']
          }
        ],
        deleted: [
          {
            required: true,
            message: this.$t('valid.deleted'),
            trigger: ['change', 'blur']
          }
        ],
        expireTime: [
          {
            required: true,
            message: this.$t('valid.expireTime'),
            trigger: ['change', 'blur']
          }
        ]
      },
      updateRules: {
        remark: [{ validator: validateAccountRemark, trigger: ['change', 'blur'] }],
        username: [
          {
            required: true,
            message: this.$t('valid.username'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.usernameRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.usernameElement'),
            trigger: ['change', 'blur']
          },
          {
            validator: validateUsername,
            trigger: ['change', 'blur']
          }
        ],
        pass: [
          {
            min: 6,
            max: 20,
            message: this.$t('valid.passRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.passElement'),
            trigger: ['change', 'blur']
          }
        ],
        quota: [
          {
            required: true,
            message: this.$t('valid.quota'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: -1,
            max: 1024000,
            message: this.$t('valid.quotaRange'),
            trigger: ['change', 'blur']
          }
        ],
        email: [
          {
            min: 4,
            max: 64,
            message: this.$t('valid.emailRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^([A-Za-z0-9_.-])+@(163.com|126.com|qq.com|gmail.com)$/,
            message: this.$t('valid.emailElement'),
            trigger: ['change', 'blur']
          }
        ],
        deleted: [
          {
            required: true,
            message: this.$t('valid.deleted'),
            trigger: ['change', 'blur']
          }
        ],
        expireTime: [
          {
            required: true,
            message: this.$t('valid.expireTime'),
            trigger: ['change', 'blur']
          }
        ]
      },
      dialogStatus: '',
      nodeAccessAccount: null,
      roleList: []
    }
  },
  created() {
    this.setRoleList()
    this.getList()
  },
  computed: {
    canViewRemarks() { return checkPermission(['sysadmin']) },
    tableColumnCount() { return this.canViewRemarks ? 8 : 7 }
  },
  watch: {
    canViewRemarks(allowed) {
      if (allowed) return
      if (Array.isArray(this.list)) this.list = this.list.map(this.accountForRole)
      this.$delete(this.temp, 'remark')
      this.dialogFormVisible = false
      this.nodeAccessAccount = null
    }
  },
  methods: {
    accountForRole(account) {
      const result = { ...account }
      if (!checkPermission(['sysadmin'])) delete result.remark
      return result
    },
    async handleResetAllAccountTraffic() {
      if (this.resettingAllTraffic || !checkPermission(['sysadmin'])) return
      this.resettingAllTraffic = true
      try {
        await MessageBox.confirm(
          this.$t('confirm.resetAllAccountTraffic'),
          this.$t('table.resetAllAccountTrafficLabel'),
          {
            confirmButtonText: this.$t('table.resetAllAccountTraffic'),
            cancelButtonText: this.$t('confirm.cancel'),
            type: 'warning'
          }
        )
        if (!checkPermission(['sysadmin'])) return
        await resetAllAccountDownloadAndUpload()
        await this.getList()
        this.$notify({
          message: this.$t('confirm.resetAllAccountTrafficSuccess'),
          type: 'success',
          duration: 2000
        })
      } catch (_) {
        // Cancellation sends no request; API failures use the shared feedback.
      } finally {
        this.resettingAllTraffic = false
      }
    },
    timePart(timestamp, index) {
      return timeStampToDate(timestamp, false).split(' ')[index] || '—'
    },
    accountUsagePct(row) {
      if (row.quota < 0) return 0
      if (!row.quota) return 100
      return Math.max(
        0,
        Math.min(
          100,
          Math.round(
            ((Number(row.upload || 0) + Number(row.download || 0)) /
              Number(row.quota)) *
              100
          )
        )
      )
    },
    checkPermission,
    getFlow,
    timeStampToDate,
    setRoleList() {
      selectRoleList().then((response) => {
        const { data } = response
        this.roleList = data
      })
    },
    getList(resetScroll = false) {
      const request = this.beginListRequest()
      this.listQuery.orderFields = this.orderFieldArr.join(',')
      return selectAccountPage(this.listQuery).then((response) => {
        if (!this.ownsListRequest(request)) return
        this.list = response.data.accounts.map(this.accountForRole)
        this.total = response.data.total
        if (resetScroll) {
          this.$nextTick(() => {
            if (this.ownsListRequest(request)) this.$refs.accountTable.scrollTop = 0
          })
        }
      }).catch(() => {
        if (!this.ownsListRequest(request)) return
        this.list = []
        this.total = 0
        this.listError = '请求失败，请重试'
      }).finally(() => this.finishListRequest(request))
    },
    resetTemp() {
      this.temp = {
        id: undefined,
        quota: 0,
        download: 0,
        upload: 0,
        username: undefined,
        pass: undefined,
        email: undefined,
        remark: undefined,
        roleId: 3,
        deleted: 0,
        expireTime: new Date().getTime(),
        createTime: new Date()
      }
    },
    handlePagination() {
      this.getList(true)
    },
    handleStatusFilter(value) {
      this.listQuery.deleted = value === '' ? undefined : value
      this.handleFilter()
    },
    handleFilter() {
      this.listQuery.pageNum = 1
      this.getList(true)
    },
    handleCreate() {
      this.resetTemp()
      setting().then((response) => {
        if (response.data.registerEnable === 1) {
          this.temp.quota = response.data.registerQuota
          this.temp.expireTime =
            new Date().getTime() + response.data.registerExpireDays * 86400000
        }
      })
      this.dialogStatus = 'create'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    handleUpdate(row) {
      this.temp = this.accountForRole(row)
      this.temp.quota = byteToMb(row.quota)
      this.originalQuota = this.temp.quota
      this.originalExpiry = row.expireTime
      this.dialogStatus = 'update'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    async handleDelete(row) {
      try {
        await MessageBox.confirm(
          this.$t('confirm.deleteUser'),
          this.$t('confirm.warn'),
          {
            confirmButtonText: this.$t('confirm.yes'),
            cancelButtonText: this.$t('confirm.cancel'),
            type: 'warning'
          }
        )
        await deleteAccountById({ id: row.id })
        await this.getList()
        this.$notify({
          message: this.$t('confirm.deleteSuccess'),
          type: 'success',
          duration: 2000
        })
      } catch (_) {
        // Cancellation sends no request; API failures use the shared feedback.
      }
    },
    createData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          const tempData = { ...this.temp }
          delete tempData.remark
          createAccount(tempData).then(() => {
            this.getList()
            this.dialogFormVisible = false
            this.$notify({
              title: 'Success',
              message: this.$t('confirm.createSuccess'),
              type: 'success',
              duration: 2000
            })
          })
        }
      })
    },
    updateData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          const tempData = Object.assign({}, this.temp)
          if (!checkPermission(['sysadmin']) || tempData.remark === undefined || tempData.remark === null) delete tempData.remark
          if (tempData.quota === this.originalQuota) delete tempData.quota
          updateAccountById(tempData).then(async ({ data }) => {
            const renewedWithoutQuota = this.originalExpiry <= this.accountNow && data && data.expireTime > this.accountNow && data.quota === 0
            await this.getList()
            this.dialogFormVisible = false
            this.$notify({
              title: 'Success',
              message: this.$t(renewedWithoutQuota ? 'accountStatus.renewedZero' : 'confirm.modifySuccess'),
              type: renewedWithoutQuota ? 'warning' : 'success',
              duration: renewedWithoutQuota ? 6000 : 2000
            })
          }).catch(() => { /* The request interceptor displays the error. */ })
        }
      })
    },
    roleFilter(roleId) {
      let role = this.roleList.find((item) => item.id === roleId)
      if (role && role.desc) {
        return role.desc
      } else {
        return ''
      }
    },
    async handleReset(row) {
      try {
        await MessageBox.confirm(
          this.$t('confirm.handleReset'),
          this.$t('confirm.warn'),
          {
            confirmButtonText: this.$t('confirm.yes'),
            cancelButtonText: this.$t('confirm.cancel'),
            type: 'warning'
          }
        )
        await resetAccountDownloadAndUpload({ id: row.id })
        await this.getList()
        this.$notify({
          message: this.$t('confirm.resetSuccess'),
          type: 'success',
          duration: 2000
        })
      } catch (_) {
        // Cancellation sends no request; API failures use the shared feedback.
      }
    },
    async handleResetLoginLimit(row) {
      if (this.resettingLoginLimitId || !checkPermission(['sysadmin']) ||
          !row || !Number.isSafeInteger(row.id) || row.id <= 0) return
      const id = row.id
      this.resettingLoginLimitId = id
      try {
        await MessageBox.confirm(
          this.$t('confirm.resetAccountLoginLimit', { username: row.username }),
          this.$t('confirm.warn'),
          {
            confirmButtonText: this.$t('confirm.yes'),
            cancelButtonText: this.$t('confirm.cancel'),
            type: 'warning'
          }
        )
        if (!checkPermission(['sysadmin'])) return
        await resetAccountLoginLimit({ id })
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.resetAccountLoginLimitSuccess'),
          type: 'success',
          duration: 2000
        })
      } catch (_) {
        // Cancellation sends no request; API failures use the shared feedback.
      } finally {
        this.resettingLoginLimitId = 0
      }
    },
    async importData({ file }) {
      const tip = this.$refs.importTip
      const valid = await tip.$refs.dataForm.validate()
      if (!valid) return false

      const retrying = this.importFailures.length > 0
      let rows
      try {
        if (retrying) {
          rows = this.importFailures.map(({ number, source }) => {
            JSON.parse(source)
            return { number, source }
          })
        } else {
          rows = accountImportRows(await file.text())
        }
      } catch (_) {
        Message.error(this.$t('accountImport.invalidJSON'))
        return false
      }

      const formData = new FormData()
      formData.append('file', retrying
        ? new File([`[${rows.map((row) => row.source).join(',')}]`], 'retry.json', { type: 'application/json' })
        : file)
      formData.append('cover', tip.temp.cover)
      let result
      try {
        const response = await importAccount(formData)
        result = response.data
      } catch (_) {
        Message.error(this.$t('accountImport.requestFailed'))
        return false
      }

      if (!this.listRequestActive) return false
      if (!result || result.imported > 0) {
        this.listQuery.pageNum = 1
        this.getList(true)
      }
      if (result?.failures.length) {
        this.importResult = { ...result, failed: result.failures.length }
        this.importFailures = result.failures.map(({ row, reason }) => ({ ...rows[row - 1], reason }))
        return false
      }
      this.importVisible = false
      this.importFailures = []
      tip.fileList = []
      if (tip.$refs.upload) tip.$refs.upload.value = ''
      const message = result
        ? this.$t('accountImport.summary', { ...result, failed: 0 })
        : this.$t('confirm.importSuccess')
      this.$notify({ message, type: 'success', duration: 4000 })
      return true
    },
    handleImport() {
      this.importVisible = true
    },
    handleExport() {
      return exportAccount().then(this.notifyDownload)
    },
    handleCreateBatch() {
      this.batchOperationDialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['batchOperationForm'].$refs['dataForm'].clearValidate()
      })
    },
    exportAccountUnused() {
      return exportAccountUnused().then(this.notifyDownload)
    },
    downloadTemplate() {
      return downloadAccountImportTemplate().then(this.notifyDownload)
    },
    notifyDownload() {
      this.$notify({
        title: 'Success',
        message: this.$t('confirm.downloadSuccess'),
        type: 'success',
        duration: 2000
      })
    },
    handleClashSubscribeForSb(row) {
      exportSubscribe({
        id: row.id,
        client: 'sing-box',
        template: 'tun'
      }).then((response) => {
        if (copy(new URL(response.data, window.location.origin).toString())) {
          Message({
            showClose: true,
            message: this.$t('confirm.urlCopySuccess').toString(),
            type: 'success'
          })
        } else {
          Message({
            showClose: true,
            message: this.$t('confirm.urlCopyFail').toString(),
            type: 'error'
          })
        }
      })
    }
  }
}
</script>

<style scoped>
.liquid-button {
  margin-left: 10px;
}
.account-remark-cell {
  min-width: 160px;
  max-width: 320px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
