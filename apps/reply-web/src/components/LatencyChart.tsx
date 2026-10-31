import { useEffect, useRef } from 'react'
import * as echarts from 'echarts'

interface LatencyChartProps {
  data: number[]
}

export const LatencyChart = ({ data }: LatencyChartProps) => {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstance = useRef<echarts.ECharts | null>(null)

  useEffect(() => {
    if (!chartRef.current) return
    chartInstance.current = echarts.init(chartRef.current)
    return () => {
      chartInstance.current?.dispose()
    }
  }, [])

  useEffect(() => {
    if (!chartInstance.current || data.length === 0) return
    chartInstance.current.setOption({
      grid: { top: 10, right: 10, bottom: 20, left: 30 },
      xAxis: { type: 'category', show: false, data: data.map((_, i) => i) },
      yAxis: {
        type: 'value',
        axisLabel: { color: '#64748b', fontSize: 9 },
        splitLine: { lineStyle: { color: '#1e293b' } },
      },
      series: [{
        type: 'line',
        data,
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        showSymbol: data.length === 1,
        itemStyle: { color: '#34d399' },
    lineStyle: { color: '#34d399', width: 2 },
    areaStyle: {
      color: {
        type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [
          { offset: 0, color: 'rgba(52, 211, 153, 0.3)' },
          { offset: 1, color: 'rgba(52, 211, 153, 0)' },
        ],
      },
    },
  }],
})
  }, [data])

  return <div ref={chartRef} style={{ height: 120 }} />
}