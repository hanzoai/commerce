riot = require 'riot'
crowdcontrol = require 'crowdcontrol'

FormView = crowdcontrol.view.form.FormView

class UserFormView extends FormView
  html: require './template.html'

module.exports = (tag, model, inputConfigs)->
  class _UserFormView extends UserFormView
    tag: tag
    model: model
    inputConfigs: inputConfigs

  new _UserFormView

  riot.mount tag

