_ = require 'underscore'
riot = require 'riot'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Api = crowdcontrol.data.Api
FormView = crowdcontrol.view.form.FormView
m = crowdcontrol.utils.mediator

class UniqueCouponFormView extends FormView
  tag: 'unique-coupon-form'
  html: require '../../templates/backend/form/unique-coupon/template.html'
  model:
    couponCode: ''
    code: ''

  # model that stores the last model queried
  uniqueCouponModel: null

  inputConfigs:[
    input('couponCode', 'Type Coupon Code Here'),
    input('code', 'Coupon Code Appears Here'),
  ]

  js: (opts)->
    super

    @api = api = Api.get('crowdstart')
    @userId = opts.userId || opts.userid

  submit: ()->
    m.trigger 'start-spin', 'user-form-save'
    @api.get("coupon/#{@model.couponCode}/code/#{@userId}").then (data)=>
      m.trigger 'stop-spin', 'user-form-save'
      couponCode = @model.couponCode
      @model = data.responseText
      @model.couponCode = couponCode
      @initFormGroup()
      riot.update()
    , ()->
      m.trigger 'stop-spin', 'user-form-save'

UniqueCouponFormView.register()
