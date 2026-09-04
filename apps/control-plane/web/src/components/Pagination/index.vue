<template>
  <div :class="{ hidden: hidden }" class="pagination-container">
    <span class="pagination-total">共 {{ total }} 条</span>
    <liquid-select
      v-if="pageSizes.length > 1"
      class="pagination-size"
      :value="pageSize"
      aria-label="每页条数"
      @input="handleSizeChange"
    >
      <option v-for="size in pageSizes" :key="size" :value="size" :label="`${size} 条/页`" />
    </liquid-select>
    <button
      class="pagination-step"
      type="button"
      aria-label="上一页"
      :disabled="currentPage <= 1"
      @click="handleCurrentChange(currentPage - 1)"
    >
      <app-icon name="arrow-left" aria-hidden="true" />
    </button>
    <button
      v-for="pageNumber in visiblePages"
      :key="pageNumber"
      class="pagination-page"
      :class="{ on: pageNumber === currentPage }"
      type="button"
      :aria-current="pageNumber === currentPage ? 'page' : null"
      @click="handleCurrentChange(pageNumber)"
    >
      {{ pageNumber }}
    </button>
    <button
      class="pagination-step"
      type="button"
      aria-label="下一页"
      :disabled="currentPage >= pageCount"
      @click="handleCurrentChange(currentPage + 1)"
    >
      <app-icon name="arrow-right" aria-hidden="true" />
    </button>
  </div>
</template>

<script>
import { scrollTo } from '@/utils/scroll-to'

export default {
  name: 'TablePagination',
  props: {
    total: {
      required: true,
      type: Number
    },
    page: {
      type: Number,
      default: 1
    },
    limit: {
      type: Number,
      default: 20
    },
    pageSizes: {
      type: Array,
      default() {
        return [10, 20, 30, 50]
      }
    },
    layout: {
      type: String,
      default: 'total, sizes, prev, pager, next, jumper'
    },
    background: {
      type: Boolean,
      default: true
    },
    autoScroll: {
      type: Boolean,
      default: true
    },
    hidden: {
      type: Boolean,
      default: false
    }
  },
  computed: {
    pageCount() {
      return Math.max(1, Math.ceil(this.total / this.pageSize))
    },
    visiblePages() {
      const start = Math.max(1, Math.min(this.currentPage - 2, this.pageCount - 4))
      const end = Math.min(this.pageCount, start + 4)
      return Array.from({ length: end - start + 1 }, (_, index) => start + index)
    },
    currentPage: {
      get() {
        return this.page
      },
      set(val) {
        this.$emit('update:page', val)
      }
    },
    pageSize: {
      get() {
        return this.limit
      },
      set(val) {
        this.$emit('update:limit', val)
      }
    }
  },
  methods: {
    handleSizeChange(val) {
      // WEB-023: persist limit first, reset to a valid page, then notify the
      // query so lists using .sync observe the new page/limit state.
      this.$emit('update:limit', val)
      const pageCount = Math.max(1, Math.ceil(this.total / val))
      const page = Math.min(this.currentPage, pageCount)
      if (page !== this.currentPage) this.$emit('update:page', page)
      this.$emit('pagination', { page, limit: val })
      if (this.autoScroll) {
        scrollTo(0)
      }
    },
    handleCurrentChange(val) {
      this.$emit('update:page', val)
      this.$emit('pagination', { page: val, limit: this.pageSize })
      if (this.autoScroll) {
        scrollTo(0)
      }
    }
  }
}
</script>

<style scoped>
.pagination-container {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  padding: 18px 4px 4px;
  color: var(--ink-3);
  background: transparent;
  font-size: 12px;
}
.pagination-container.hidden {
  display: none;
}
.pagination-total {
  margin-right: 6px;
  white-space: nowrap;
}
.pagination-size {
  width: 108px;
  margin-right: 4px;
}
.pagination-step,
.pagination-page {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 34px;
  height: 34px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 11px;
  color: var(--ink-2);
  background: var(--glass-soft);
  cursor: pointer;
}
.pagination-page.on {
  color: var(--on-accent);
  background: var(--accent);
  box-shadow: var(--shadow-soft);
}
.pagination-step:disabled {
  cursor: not-allowed;
  opacity: 0.38;
}
@media (max-width: 640px) {
  .pagination-container {
    justify-content: center;
    flex-wrap: wrap;
  }
  .pagination-size {
    display: none;
  }
}
</style>
