Integration = require '../integration'

input = require '../../../form/input'

class HeapAnalytics extends Integration
  tag: 'heap-integration'
  type: 'heap-analytics'
  html: require '../../../templates/backend/widget/integrations/analytics/heap.html'
  img: '/img/integrations/heap.png'
  alt: 'Heap Analytics'
  text: 'Heap Analytics'

  inputConfigs: [
    input('id', '123456789', 'required')
    input('sampling', '', 'numeric')
  ]

HeapAnalytics.register()

module.exports = HeapAnalytics
