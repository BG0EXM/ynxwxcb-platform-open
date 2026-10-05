<template>
  <div class="mobile-home">
    <van-nav-bar title="工作台" />
    <div class="content">
      <van-card
        desc="关于召开2026年宣传思想文化工作推进会的通知"
        title="待办任务：会议通知"
        thumb="https://fastly.jsdelivr.net/npm/@vant/assets/icon-demo.png"
      >
        <template #tags>
          <van-tag plain type="danger">紧急</van-tag>
          <van-tag plain type="primary">公文</van-tag>
        </template>
        <template #footer>
          <van-button size="mini" type="primary">去处理</van-button>
        </template>
      </van-card>

      <van-card
        desc="王某某的请假申请（3天）"
        title="待办审批：请假申请"
        thumb="https://fastly.jsdelivr.net/npm/@vant/assets/icon-demo.png"
      >
        <template #tags>
          <van-tag plain type="primary">审批</van-tag>
        </template>
        <template #footer>
          <van-button size="mini" type="primary">去处理</van-button>
        </template>
      </van-card>
    </div>
  </div>
</template>

<script>
import { NavBar, Card, Tag, Button } from 'vant'

export default {
  name: 'MobileHome',
  components: {
    [NavBar.name]: NavBar,
    [Card.name]: Card,
    [Tag.name]: Tag,
    [Button.name]: Button
  }
}
</script>

<style scoped>
.mobile-home {
  background-color: #f7f8fa;
  min-height: 100%;
}
.content {
  padding: 10px;
}
.van-card {
  margin-bottom: 10px;
  background-color: #fff;
  border-radius: 8px;
}
</style>
