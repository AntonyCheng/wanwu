import {useQiankun} from './qiankunUtil'
import Vue from 'vue'
import App from './App.vue'
import router from './router'
import { store } from './store'
import { i18n } from './lang'
import './router/permission'

import ElementUi from 'element-ui'
import moment from 'moment';
import 'element-ui/lib/theme-chalk/index.css';
Vue.use(ElementUi, {
    i18n: (key, value) => i18n.t(key, value), // 根据选的语言切换 Element-ui 的语言
})
import  '@/style/index.scss'
import { config, basePath } from './utils/config'

require('./utils/rem.js')
import {guid} from '@/utils/util'
Vue.prototype.$config = config
Vue.prototype.$basePath = basePath
Vue.prototype.$guid = guid
Vue.prototype.$copy = function copy(text){
    var textareaEl = document.createElement('textarea');
    textareaEl.setAttribute('readonly', 'readonly'); // 防止手机上弹出软键盘
    textareaEl.value = text;
    document.body.appendChild(textareaEl);
    textareaEl.select();
    var res = document.execCommand('copy');
    document.body.removeChild(textareaEl);
    return res;
}

Vue.config.productionTip = false

// 定义时间格式全局过滤器
Vue.filter('dateFormat', function(daraStr, pattern = 'YYYY-MM-DD HH:mm:ss') {
  return moment(daraStr).format(pattern)
})

const vueApp = new Vue({
  router,
  store,
  i18n,
  render: function (h) { return h(App) }
}).$mount('#app')

/*vueApp.$nextTick(() => {
    useQiankun()
})*/
