Page = require './page'

class Coupons extends Page
  tag: 'page-coupons'
  icon: 'glyphicon glyphicon-tag'
  name: 'Coupons'
  html: require '../../templates/dash/site/pages/coupons.html'

  collection: 'coupons'

Coupons.register()

module.exports = Coupons
