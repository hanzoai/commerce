_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class CouponForm extends Form
  tag: 'coupon-form'
  redirectPath: 'coupons'
  path: 'coupon'
  model:
    productId: '_'

  inputConfigs: [
    input('id', '', 'static'),
    input('name', 'Name', 'required')
    input('code', 'Coupon Code', 'required unique unique-api:coupon')
    input('type', 'Coupon Type', 'coupon-type-select')
    input('amount', 'Coupon Amount', 'money'),
    input('productId', 'Select a Product', 'product-type-select')

    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

  _submit: (event)->
    @model.productId = '' if @model.productId == '_'
    super

  loadData: (model)->
    super
    model.productId = '_' if model.productId == ''
    @inputConfigs[2].hints['unique-exception'] = model.code

CouponForm.register()

module.exports = CouponForm
