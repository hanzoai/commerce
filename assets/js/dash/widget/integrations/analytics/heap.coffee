Integration = require '../integration'

input = require '../../../form/input'

class HeapAnalytics extends Integration
  tag: 'heap-integration'
  type: 'analytics-heap'
  html: require '../../../templates/dash/widget/integrations/analytics/heap.html'
  img: '/img/integrations/heap.png'
  alt: 'Heap Analytics'
  text: 'Heap Analytics'

  inputConfigs: [
    input('data.id', '123456789', 'required')
    input('data.sampling', '', 'numeric')
  ]

HeapAnalytics.register()

module.exports = HeapAnalytics
