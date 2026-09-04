<template>
  <div class="traffic-period-filter">
    <div
      class="seg dashboard-segment dashboard-segment-3"
      role="group"
      :aria-label="ariaLabel"
    >
      <button
        v-for="option in periodOptions"
        :key="option.value"
        type="button"
        :class="{ on: value.period === option.value }"
        :aria-pressed="String(value.period === option.value)"
        @click="setPeriod(option.value)"
      >
        {{ option.label }}
      </button>
    </div>
    <liquid-date-picker
      v-if="value.period === 'month'"
      :value="selectedMonth"
      class="traffic-period-filter__date"
      type="month"
      value-format="yyyy-MM"
      :clearable="false"
      :placeholder="$t('traffic.selectMonth')"
      @input="setDate('month', $event)"
    />
    <liquid-date-picker
      v-if="value.period === 'day'"
      :value="selectedDay"
      class="traffic-period-filter__date"
      type="date"
      value-format="yyyy-MM-dd"
      :clearable="false"
      :placeholder="$t('traffic.selectDay')"
      @input="setDate('day', $event)"
    />
  </div>
</template>

<script>
function currentDates() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return {
    month: `${year}-${month}`,
    day: `${year}-${month}-${day}`
  }
}

export default {
  name: 'TrafficPeriodFilter',
  props: {
    value: {
      type: Object,
      required: true
    },
    ariaLabel: {
      type: String,
      required: true
    }
  },
  data() {
    const dates = currentDates()
    return {
      selectedMonth:
        this.value.period === 'month' && this.value.date
          ? this.value.date
          : dates.month,
      selectedDay:
        this.value.period === 'day' && this.value.date
          ? this.value.date
          : dates.day
    }
  },
  computed: {
    periodOptions() {
      return [
        { value: 'total', label: this.$t('traffic.rankAll') },
        { value: 'month', label: this.$t('traffic.rankMonth') },
        { value: 'day', label: this.$t('traffic.rankDay') }
      ]
    }
  },
  methods: {
    emitValue(period, date) {
      const value = { period, date }
      this.$emit('input', value)
      this.$emit('change', value)
    },
    setPeriod(period) {
      if (period === this.value.period) return
      const date =
        period === 'month'
          ? this.selectedMonth
          : period === 'day'
          ? this.selectedDay
          : ''
      this.emitValue(period, date)
    },
    setDate(period, date) {
      if (!date) return
      if (period === 'month') this.selectedMonth = date
      else this.selectedDay = date
      if (this.value.period === period && this.value.date !== date) {
        this.emitValue(period, date)
      }
    }
  }
}
</script>

<style scoped>
.traffic-period-filter {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  min-width: 0;
}

.traffic-period-filter__date {
  width: 170px;
}

@media (max-width: 760px) {
  .traffic-period-filter,
  .traffic-period-filter__date {
    width: 100%;
  }
}
</style>
