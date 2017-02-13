_ = require 'underscore'
riot = require 'riot'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Api = crowdcontrol.data.Api
FormView = crowdcontrol.view.form.FormView
m = crowdcontrol.utils.mediator

class ResetPasswordFormView extends FormView
  tag: 'reset-password-form'
  html: require '../../templates/backend/form/random-password/template.html'
  model:
    password: ''

  # model that stores the last model queried
  resetModel: null

  inputConfigs:[
    input('password', 'Password Appears Here'),
  ]

  js: (opts)->
    super

    @api = api = Api.get('crowdstart')
    @userId = opts.userId || opts.userid

  submit: ()->
    m.trigger 'start-spin', 'user-form-save'
    @api.get("user/#{@userId}/password/reset").then (data)=>
      m.trigger 'stop-spin', 'user-form-save'
      @model = data.responseText
      @initFormGroup()
      riot.update()
    , ()->
      m.trigger 'stop-spin', 'user-form-save'

ResetPasswordFormView.register()
