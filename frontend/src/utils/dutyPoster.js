import dayjs from 'dayjs'

/**
 * 计算选定日期所在自然周（周一至周日）的详细信息
 * @param {string|Date} targetDate 选中的任意一天
 */
export function getWeekRangeInfo(targetDate) {
  const d = dayjs(targetDate || new Date())
  const dayOfWeek = d.day() // 0=周日, 1=周一, ..., 6=周六
  const mondayOffset = dayOfWeek === 0 ? -6 : -(dayOfWeek - 1)
  const monday = d.add(mondayOffset, 'day')

  const weekNames = ['星期一', '星期二', '星期三', '星期四', '星期五', '星期六', '星期日']
  const weekShortNames = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

  const weekDays = []
  for (let i = 0; i < 7; i++) {
    const cur = monday.add(i, 'day')
    const curStr = cur.format('YYYY-MM-DD')
    weekDays.push({
      dateStr: curStr,
      dateObj: cur,
      monthDay: cur.format('M月D日'),
      monthDayShort: cur.format('MM-DD'),
      weekName: weekNames[i],
      weekShort: weekShortNames[i],
      isWeekend: i >= 5,
      isToday: curStr === dayjs().format('YYYY-MM-DD')
    })
  }

  // 估算年内周次
  const startOfYear = dayjs(`${monday.format('YYYY')}-01-01`)
  const diffDays = monday.diff(startOfYear, 'day')
  const weekNumber = Math.max(1, Math.floor(diffDays / 7) + 1)

  const startDateStr = weekDays[0].dateStr
  const endDateStr = weekDays[6].dateStr
  const rangeTitle = `${weekDays[0].monthDay}（${weekDays[0].weekShort}）至 ${weekDays[6].monthDay}（${weekDays[6].weekShort}）`
  const weekNumberText = `${monday.format('YYYY')}年第${weekNumber}周`

  return {
    mondayDate: startDateStr,
    sundayDate: endDateStr,
    weekDays,
    weekNumber,
    year: monday.format('YYYY'),
    rangeTitle,
    weekNumberText
  }
}

/**
 * 绘制圆角矩形辅助函数（兼容所有 Canvas 环境）
 */
function drawRoundRect(ctx, x, y, width, height, radius) {
  let r = { tl: 0, tr: 0, br: 0, bl: 0 }
  if (typeof radius === 'number') {
    r = { tl: radius, tr: radius, br: radius, bl: radius }
  } else if (radius) {
    r = { tl: radius.tl || 0, tr: radius.tr || 0, br: radius.br || 0, bl: radius.bl || 0 }
  }
  ctx.beginPath()
  ctx.moveTo(x + r.tl, y)
  ctx.lineTo(x + width - r.tr, y)
  ctx.quadraticCurveTo(x + width, y, x + width, y + r.tr)
  ctx.lineTo(x + width, y + height - r.br)
  ctx.quadraticCurveTo(x + width, y + height, x + width - r.br, y + height)
  ctx.lineTo(x + r.bl, y + height)
  ctx.quadraticCurveTo(x, y + height, x, y + height - r.bl)
  ctx.lineTo(x, y + r.tl)
  ctx.quadraticCurveTo(x, y, x + r.tl, y)
  ctx.closePath()
}

/**
 * 绘制五角星辅助函数
 */
function drawStar(ctx, cx, cy, spikes, outerRadius, innerRadius) {
  let rot = (Math.PI / 2) * 3
  let x = cx
  let y = cy
  const step = Math.PI / spikes
  ctx.beginPath()
  ctx.moveTo(cx, cy - outerRadius)
  for (let i = 0; i < spikes; i++) {
    x = cx + Math.cos(rot) * outerRadius
    y = cy + Math.sin(rot) * outerRadius
    ctx.lineTo(x, y)
    rot += step
    x = cx + Math.cos(rot) * innerRadius
    y = cy + Math.sin(rot) * innerRadius
    ctx.lineTo(x, y)
    rot += step
  }
  ctx.lineTo(cx, cy - outerRadius)
  ctx.closePath()
}

/**
 * 绘制红色政务印鉴章
 */
function drawOfficialStamp(ctx, x, y) {
  ctx.save()
  ctx.translate(x, y)
  ctx.rotate((-5 * Math.PI) / 180) // 微倾斜5度更具真实盖印质感
  ctx.globalAlpha = 0.46

  const stampColor = '#dc2626'
  const R = 46

  // 1. 双圆环边框
  ctx.strokeStyle = stampColor
  ctx.lineWidth = 2.4
  ctx.beginPath()
  ctx.arc(0, 0, R, 0, Math.PI * 2)
  ctx.stroke()

  ctx.lineWidth = 0.8
  ctx.beginPath()
  ctx.arc(0, 0, R - 3.5, 0, Math.PI * 2)
  ctx.stroke()

  // 2. 中心五角星
  ctx.fillStyle = stampColor
  drawStar(ctx, 0, -2, 5, 12, 5)
  ctx.fill()

  // 3. 沿上圆弧排布文字：“中共伊宁县委宣传部”
  const arcText = '中共伊宁县委宣传部'
  const textRadius = R - 13
  const startAngle = Math.PI + 0.35
  const endAngle = Math.PI * 2 - 0.35
  const angleStep = (endAngle - startAngle) / (arcText.length - 1)

  ctx.font = 'bold 9.5px "SimSun", "STSong", "PingFang SC", sans-serif'
  ctx.fillStyle = stampColor
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'

  for (let i = 0; i < arcText.length; i++) {
    const angle = startAngle + i * angleStep
    ctx.save()
    ctx.rotate(angle + Math.PI / 2)
    ctx.translate(0, -textRadius)
    ctx.fillText(arcText[i], 0, 0)
    ctx.restore()
  }

  // 4. 下方横排“专用章”或“办公室印”
  ctx.font = 'bold 10px "SimSun", "STSong", "PingFang SC", sans-serif'
  ctx.fillText('办公室印', 0, 22)

  ctx.restore()
}

/**
 * 生成选定周值守排班 Canvas
 * @param {Array} weekDaysWithData 包含每天排班列表的 7 天数据
 * @param {Object} meta 元信息与配置项
 * @returns {HTMLCanvasElement}
 */
export function generateDutyPosterCanvas(weekDaysWithData, meta = {}) {
  const {
    theme = 'red', // 'red' | 'blue'
    showNotice = true,
    weekRangeText = '',
    weekNumberText = '',
    generatedTime = dayjs().format('YYYY-MM-DD HH:mm')
  } = meta

  // 主题配色
  const colors = theme === 'blue' ? {
    primary: '#1e3a8a',
    primaryDark: '#172554',
    headerBg: '#1e3a8a',
    gradientStart: '#1e3a8a',
    gradientEnd: '#2563eb',
    accent: '#0284c7',
    border: '#cbd5e1',
    tableBorder: '#e2e8f0',
    titleColor: '#0f172a'
  } : {
    primary: '#991b1b',
    primaryDark: '#7f1d1d',
    headerBg: '#991b1b',
    gradientStart: '#991b1b',
    gradientEnd: '#dc2626',
    accent: '#b45309',
    border: '#fca5a5',
    tableBorder: '#fecaca',
    titleColor: '#1e293b'
  }

  // 尺寸规划
  const W = 1000
  const PAD = 36
  const CONTENT_W = W - PAD * 2

  // 计算表格各行高度
  const rowHeights = weekDaysWithData.map(day => {
    const count = (day.schedules || []).length
    if (count <= 1) return 76
    if (count === 2) return 106
    return 136
  })
  const tableRowsTotalHeight = rowHeights.reduce((acc, h) => acc + h, 0)

  const headerHeight = 175
  const noticeHeight = showNotice ? 66 : 0
  const tableHeaderHeight = 44
  const footerHeight = 85
  const H = PAD + headerHeight + (showNotice ? noticeHeight + 14 : 10) + tableHeaderHeight + tableRowsTotalHeight + footerHeight + PAD

  // 高分视网膜缩放
  const SCALE = 2
  const canvas = document.createElement('canvas')
  canvas.width = W * SCALE
  canvas.height = H * SCALE
  const ctx = canvas.getContext('2d')
  ctx.scale(SCALE, SCALE)

  // 1. 底板背景
  ctx.fillStyle = '#f8fafc'
  ctx.fillRect(0, 0, W, H)

  // 2. 主卡片框（白色圆角，微阴影质感）
  const cardX = PAD
  const cardY = PAD
  const cardW = CONTENT_W
  const cardH = H - PAD * 2

  ctx.fillStyle = '#ffffff'
  drawRoundRect(ctx, cardX, cardY, cardW, cardH, 14)
  ctx.fill()
  ctx.strokeStyle = '#e2e8f0'
  ctx.lineWidth = 1.2
  ctx.stroke()

  // 顶部渐变装饰条
  const topBarGrad = ctx.createLinearGradient(cardX, cardY, cardX + cardW, cardY)
  topBarGrad.addColorStop(0, colors.gradientStart)
  topBarGrad.addColorStop(1, colors.gradientEnd)
  ctx.fillStyle = topBarGrad
  drawRoundRect(ctx, cardX, cardY, cardW, 8, { tl: 14, tr: 14, br: 0, bl: 0 })
  ctx.fill()

  // 3. 页头部分
  let curY = cardY + 36

  // 机构标识
  ctx.font = 'bold 16px "PingFang SC", "Microsoft YaHei", sans-serif'
  ctx.fillStyle = colors.primary
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText('中 共 伊 宁 县 委 宣 传 部', W / 2, curY)

  // 主标题
  curY += 40
  ctx.font = 'bold 32px "SimSun", "STSong", "PingFang SC", sans-serif'
  ctx.fillStyle = colors.titleColor
  ctx.fillText('值 班 值 守 安 排 表', W / 2, curY)

  // 周期副标题
  curY += 34
  ctx.font = '14px "PingFang SC", "Microsoft YaHei", sans-serif'
  ctx.fillStyle = '#475569'
  ctx.fillText(`值守周期：${weekRangeText} · ${weekNumberText}`, W / 2, curY)

  // 标题下三段式雅致装饰线
  curY += 22
  const lineW = 380
  const lineStartX = (W - lineW) / 2
  ctx.strokeStyle = '#cbd5e1'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(lineStartX, curY)
  ctx.lineTo(lineStartX + 160, curY)
  ctx.stroke()

  ctx.beginPath()
  ctx.moveTo(lineStartX + lineW - 160, curY)
  ctx.lineTo(lineStartX + lineW, curY)
  ctx.stroke()

  // 中心小菱形
  ctx.fillStyle = colors.accent
  ctx.beginPath()
  ctx.moveTo(W / 2, curY - 4)
  ctx.lineTo(W / 2 + 5, curY)
  ctx.lineTo(W / 2, curY + 4)
  ctx.lineTo(W / 2 - 5, curY)
  ctx.closePath()
  ctx.fill()

  // 4. 值守纪律提示栏（若开启）
  curY += 18
  if (showNotice) {
    const noticeX = cardX + 24
    const noticeW = cardW - 48
    ctx.fillStyle = '#fefce8'
    drawRoundRect(ctx, noticeX, curY, noticeW, noticeHeight, 8)
    ctx.fill()
    ctx.strokeStyle = '#fde047'
    ctx.lineWidth = 1
    ctx.stroke()

    // 提示标题
    ctx.font = 'bold 13px "PingFang SC", "Microsoft YaHei", sans-serif'
    ctx.fillStyle = '#854d0e'
    ctx.textAlign = 'left'
    ctx.fillText('🔔 【值守纪律与要求】', noticeX + 16, curY + 22)

    // 提示内容
    ctx.font = '13px "PingFang SC", "Microsoft YaHei", sans-serif'
    ctx.fillStyle = '#713f12'
    ctx.fillText('1. 值守干部当天值守至晚上 21:00（收文结束）； 2. 遇紧急突发公文或情况，第一时间向带班领导报告。', noticeX + 16, curY + 44)

    curY += noticeHeight + 16
  }

  // 5. 排班主体表格
  const tableX = cardX + 24
  const tableW = cardW - 48

  // 表格列宽配置
  const colWDate = 175
  const colWLoc = 150
  const colWPerson = 270
  const colWNote = tableW - colWDate - colWLoc - colWPerson // 约 285px

  const xDate = tableX
  const xLoc = xDate + colWDate
  const xPerson = xLoc + colWLoc
  const xNote = xPerson + colWPerson

  // 5.1 表头
  ctx.fillStyle = colors.headerBg
  drawRoundRect(ctx, tableX, curY, tableW, tableHeaderHeight, { tl: 8, tr: 8, br: 0, bl: 0 })
  ctx.fill()

  ctx.font = 'bold 15px "PingFang SC", "Microsoft YaHei", sans-serif'
  ctx.fillStyle = '#ffffff'
  ctx.textBaseline = 'middle'

  ctx.textAlign = 'center'
  ctx.fillText('日期 / 星期', xDate + colWDate / 2, curY + tableHeaderHeight / 2)
  ctx.fillText('值守地点', xLoc + colWLoc / 2, curY + tableHeaderHeight / 2)
  ctx.fillText('值守人员', xPerson + colWPerson / 2, curY + tableHeaderHeight / 2)

  ctx.textAlign = 'left'
  ctx.fillText('值守要求与备注', xNote + 20, curY + tableHeaderHeight / 2)

  curY += tableHeaderHeight

  // 5.2 循环绘制周一至周日 7 天
  weekDaysWithData.forEach((day, index) => {
    const rowH = rowHeights[index]
    const isEven = index % 2 === 1
    const isWeekend = day.isWeekend
    const isToday = day.isToday
    const isLast = index === weekDaysWithData.length - 1

    // 行背景
    if (isToday) {
      ctx.fillStyle = '#eff6ff' // 今日淡青蓝高亮
    } else if (isWeekend) {
      ctx.fillStyle = '#fffdf5' // 周末暖米黄
    } else if (isEven) {
      ctx.fillStyle = '#f8fafc' // 偶数行淡灰
    } else {
      ctx.fillStyle = '#ffffff'
    }

    if (isLast) {
      drawRoundRect(ctx, tableX, curY, tableW, rowH, { tl: 0, tr: 0, br: 8, bl: 8 })
      ctx.fill()
    } else {
      ctx.fillRect(tableX, curY, tableW, rowH)
    }

    // 今日左侧高亮细色条
    if (isToday) {
      ctx.fillStyle = '#2563eb'
      ctx.fillRect(tableX, curY, 4, rowH)
    }

    // 单元格中线 Y
    const rowMidY = curY + rowH / 2

    // (A) 日期与星期单元格
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'

    // 日期大字
    ctx.font = 'bold 18px "PingFang SC", "Microsoft YaHei", sans-serif'
    ctx.fillStyle = isToday ? '#1d4ed8' : '#0f172a'
    ctx.fillText(day.monthDay, xDate + colWDate / 2, rowMidY - 10)

    // 星期小字
    ctx.font = '13px "PingFang SC", "Microsoft YaHei", sans-serif'
    ctx.fillStyle = isWeekend ? '#d97706' : '#64748b'
    ctx.fillText(day.weekName, xDate + colWDate / 2, rowMidY + 12)

    // 徽标（今日/周末）
    if (isToday) {
      const tagW = 32
      const tagH = 18
      const tagX = xDate + 16
      const tagY = curY + 10
      ctx.fillStyle = '#2563eb'
      drawRoundRect(ctx, tagX, tagY, tagW, tagH, 4)
      ctx.fill()
      ctx.font = 'bold 11px "PingFang SC", sans-serif'
      ctx.fillStyle = '#ffffff'
      ctx.fillText('今日', tagX + tagW / 2, tagY + tagH / 2)
    } else if (isWeekend) {
      const tagW = 32
      const tagH = 18
      const tagX = xDate + 16
      const tagY = curY + 10
      ctx.fillStyle = '#fef3c7'
      drawRoundRect(ctx, tagX, tagY, tagW, tagH, 4)
      ctx.fill()
      ctx.strokeStyle = '#fcd34d'
      ctx.lineWidth = 0.8
      ctx.stroke()
      ctx.font = 'bold 11px "PingFang SC", sans-serif'
      ctx.fillStyle = '#b45309'
      ctx.fillText('周末', tagX + tagW / 2, tagY + tagH / 2)
    }

    // (B) 排班人员、地点与备注
    const schedules = day.schedules || []

    if (schedules.length === 0) {
      // 未排班
      ctx.textAlign = 'center'
      ctx.font = '14px "PingFang SC", sans-serif'
      ctx.fillStyle = '#94a3b8'
      ctx.fillText('—', xLoc + colWLoc / 2, rowMidY)

      ctx.font = 'italic 14px "PingFang SC", sans-serif'
      ctx.fillText('（未安排值守）', xPerson + colWPerson / 2, rowMidY)

      ctx.textAlign = 'left'
      ctx.font = '13px "PingFang SC", sans-serif'
      ctx.fillText('在岗待命', xNote + 20, rowMidY)
    } else {
      // 有排班人员（支持 1~3 人）
      const itemH = rowH / schedules.length
      schedules.forEach((s, sIdx) => {
        const itemMidY = curY + itemH * sIdx + itemH / 2
        const isDaWangYuan = s.is_dawangyuan === 1

        // 1. 地点胶囊徽章
        const badgeW = 96
        const badgeH = 26
        const badgeX = xLoc + (colWLoc - badgeW) / 2
        const badgeY = itemMidY - badgeH / 2

        if (isDaWangYuan) {
          ctx.fillStyle = '#fee2e2'
          drawRoundRect(ctx, badgeX, badgeY, badgeW, badgeH, 6)
          ctx.fill()
          ctx.strokeStyle = '#fca5a5'
          ctx.lineWidth = 1
          ctx.stroke()

          ctx.font = 'bold 12px "PingFang SC", sans-serif'
          ctx.fillStyle = '#b91c1c'
          ctx.textAlign = 'center'
          ctx.fillText('县委大院值守', badgeX + badgeW / 2, itemMidY)
        } else {
          ctx.fillStyle = '#f1f5f9'
          drawRoundRect(ctx, badgeX, badgeY, badgeW, badgeH, 6)
          ctx.fill()
          ctx.strokeStyle = '#cbd5e1'
          ctx.lineWidth = 1
          ctx.stroke()

          ctx.font = 'bold 12px "PingFang SC", sans-serif'
          ctx.fillStyle = '#334155'
          ctx.textAlign = 'center'
          ctx.fillText('部机关值守', badgeX + badgeW / 2, itemMidY)
        }

        // 2. 人员姓名
        ctx.textAlign = 'center'
        ctx.font = 'bold 17px "PingFang SC", "Microsoft YaHei", sans-serif'
        ctx.fillStyle = '#0f172a'
        let nameText = s.user_name || '值班干部'
        if (s.department_name) {
          nameText += ` (${s.department_name})`
        }
        ctx.fillText(nameText, xPerson + colWPerson / 2, itemMidY)

        // 3. 备注要求
        ctx.textAlign = 'left'
        ctx.font = '13px "PingFang SC", "Microsoft YaHei", sans-serif'
        ctx.fillStyle = s.note ? '#1e293b' : '#64748b'
        const noteText = s.note || '在岗值守至 21:00 收文完毕'
        ctx.fillText(noteText, xNote + 20, itemMidY)
      })
    }

    // 单元格下划线
    if (!isLast) {
      ctx.strokeStyle = '#e2e8f0'
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(tableX, curY + rowH)
      ctx.lineTo(tableX + tableW, curY + rowH)
      ctx.stroke()
    }

    curY += rowH
  })

  // 表格全外框描边
  ctx.strokeStyle = '#cbd5e1'
  ctx.lineWidth = 1.2
  drawRoundRect(ctx, tableX, cardY + 36 + 40 + 34 + 22 + (showNotice ? noticeHeight + 16 : 0) + 18, tableW, tableHeaderHeight + tableRowsTotalHeight, 8)
  ctx.stroke()

  // 6. 底部页脚
  curY = cardY + cardH - 52

  ctx.strokeStyle = '#e2e8f0'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(cardX + 24, curY)
  ctx.lineTo(cardX + cardW - 24, curY)
  ctx.stroke()

  curY += 28

  // 左侧印发说明
  ctx.font = '13px "PingFang SC", "Microsoft YaHei", sans-serif'
  ctx.fillStyle = '#64748b'
  ctx.textAlign = 'left'
  ctx.textBaseline = 'middle'
  ctx.fillText('伊宁县委宣传部部务工作平台', cardX + 26, curY)

  // 右侧制表时间
  ctx.textAlign = 'right'
  ctx.fillText(`制表时间：${generatedTime}`, cardX + cardW - 130, curY)

  // 7. 盖上逼真机关公章（位于右下角时间旁偏上位置）
  drawOfficialStamp(ctx, cardX + cardW - 80, curY - 14)

  return canvas
}
