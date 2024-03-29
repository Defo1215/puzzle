<script lang="ts" setup>
import TopBar from "@/components/top-bar.vue";
import { InfoRound } from "@vicons/material";
import { Raw, onMounted, ref } from "vue";
import { useUserStore } from "@/store/user";
import { useNotificationStore } from "@/store/notification";
import { notificationRequest } from "api/notification";
import { useMessage } from "naive-ui";
import { useScrollTo } from "@/utils/useScrollTo";
import type { NotificationResp } from "@/types/notification";

const Message = useMessage();

const userStore = useUserStore();
const notificationStore = useNotificationStore();

const NotificationIcon: Record<string, Raw<any>> = {
  InfoRound: InfoRound,
};

const currentPage = ref(1);

// 处理通知已读
const handleNotificationRead = async (data: NotificationResp) => {
  if (data.readStatus === 2) return;

  const {
    data: { msg, code },
  } = await notificationRequest.update({ id: data.id, readStatus: 2 });

  if (code !== 200) {
    Message.error(msg);
  }

  notificationStore.fetchNotifications(
    {
      pagination: {
        page: 1,
        pageSize: 10 * currentPage.value,
      },
    },
    "overwrite"
  );

  notificationStore.fetchUnreadCount();
};

// 处理全部已读
const handleReadAll = async () => {
  const {
    data: { msg, code, data: readAllResp },
  } = await notificationRequest.readAll({
    userId: userStore.getUser.id,
  });

  if (code === 200) {
    Message.success(readAllResp);
  } else {
    Message.error(msg);
  }

  notificationStore.fetchNotifications(
    {
      pagination: {
        page: 1,
        pageSize: 10 * currentPage.value,
      },
    },
    "overwrite"
  );

  notificationStore.fetchUnreadCount();

  currentPage.value = 1;
};

// 获取通知
const fetchNotifications = async (type: string) => {
  await notificationStore.fetchNotifications(
    {
      pagination: {
        page: currentPage.value,
        pageSize: 10,
      },
    },
    type
  );

  currentPage.value++;
};

useScrollTo(200, () => {
  if (notificationStore.getNotificationTotal < 10 * (currentPage.value - 1)) {
    return;
  }

  fetchNotifications("append");
});
onMounted(() => {
  fetchNotifications("overwrite");
});
</script>

<template>
  <div>
    <top-bar pt-4 title="通知">
      <template #right>
        <n-button
          class="mr-4"
          type="primary"
          secondary
          strong
          @click="handleReadAll"
          v-if="notificationStore.getNotificationTotal"
        >
          全部已读
        </n-button>
      </template>
    </top-bar>

    <div class="p-4" v-if="notificationStore.getNotificationTotal">
      <div
        v-for="(data, index) in notificationStore.getNotificationList"
        :key="index"
        @click="handleNotificationRead(data)"
      >
        <!-- 通知标注 -->
        <div class="flex justify-end h-2">
          <n-badge
            class="scale-150"
            dot
            processing
            :offset="[3.5, 4]"
            :show="data.readStatus === 1"
          />
        </div>
        <!-- 通知内容 -->
        <div
          class="p-4 w-full bg-white shadow-sm rounded-xl mb-2 transition-all duration-300"
          :class="{
            'text-gray': data.readStatus === 2,
          }"
        >
          <!-- 标题与时间 -->
          <div class="flex justify-between items-center">
            <div class="flex justify-between items-center gap-2">
              <n-icon
                :size="24"
                :component="NotificationIcon[data.notificationTypeInfo.icon]"
              />
              <div class="text-4.5 font-bold">
                {{ data.notificationTypeInfo?.name }}
              </div>
            </div>
            <div class="text-4 text-gray">
              {{ new Date(data.updatedAt).toLocaleDateString() }}
            </div>
          </div>
          <!-- 内容 -->
          <div class="mt-2 indent-4 text-4">{{ data.content }}</div>
        </div>
      </div>
    </div>

    <n-result
      class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-4/5"
      status="418"
      title="暂无通知"
      description="等待我们的消息吧！"
      v-if="notificationStore.getNotificationTotal === 0 && userStore.getToken"
    />

    <n-result
      class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-4/5"
      status="403"
      title="禁止访问"
      description="这里只有登录用户才能访问噢!"
      v-if="!userStore.getToken"
    />
  </div>
</template>

<style scoped></style>
