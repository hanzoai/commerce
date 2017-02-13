Page = require './page'

class Coupon extends Page
  tag: 'page-coupon'
  icon: 'glyphicon glyphicon-tag'
  name: 'Coupon'
  html: require '../../templates/dash/site/pages/coupon.html'

  collection: 'coupon'

Coupon.register()

module.exports = Coupon
