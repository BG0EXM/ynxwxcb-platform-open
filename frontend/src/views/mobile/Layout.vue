<template>
  <div class="mobile-layout">
    <router-view class="mobile-content"></router-view>
    <van-tabbar v-model="active" route>
      <van-tabbar-item replace to="/mobile/home" icon="home-o">首页</van-tabbar-item>
      <van-tabbar-item replace to="/mobile/contacts" icon="friends-o">通讯录</van-tabbar-item>
      <van-tabbar-item replace to="/mobile/apply" icon="add-o">快捷申请</van-tabbar-item>
      <van-tabbar-item replace to="/mobile/mine" icon="user-o">我的</van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<script>
import { ref } from 'vue'
import { Tabbar, TabbarItem } from 'vant'
import 'vant/lib/index.css'

export default {
  name: 'MobileLayout',
  components: {
    [Tabbar.name]: Tabbar,
    [TabbarItem.name]: TabbarItem
  },
  setup() {
    const active = ref(0)
    return { active }
  }
}
</script>

<style scoped>
.mobile-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
}
.mobile-content {
  flex: 1;
  overflow-y: auto;
  padding-bottom: 50px;
}
</style>
