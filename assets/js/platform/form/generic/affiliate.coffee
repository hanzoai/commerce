_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'
Events = crowdcontrol.Events

Api = crowdcontrol.data.Api

class AffiliateForm extends Form
  tag: 'affiliate-form'
  redirectPath: 'users'
  path: 'affiliate'
  model:
    enabled: false
    commision:
      percent: 0
      flat: 0

  assignToUserFn: ()->

  inputConfigs: [
    input('commission.percent', 'Percent Fee', 'percent'),
    input('commission.flat', 'Flat Fee', 'money'),
    input('enabled', '', 'switch'),
  ]

  constructor: ->
    @model =
      enabled: false
      commision:
        percent: 0
        flat: 0

    super

  js: (opts)->
    @assignToUserFn = opts.assigntouserfn
    if !@userObs
      @userObs = opts.userobs
      @userObs.on "#{Events.Form.Prefill}", (model)=>
        @opts.id = @model.id = model.affiliateId
        @model.userId = model.id
        @js @opts

    super

  loadData: (model)->
    super

  _submit: (event)->
    super(event).then ()=>
      @assignToUserFn @model

AffiliateForm.register()

module.exports = AffiliateForm
