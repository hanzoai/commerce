riot = require 'riot'
crowdcontrol = require 'crowdcontrol'

FormView = crowdcontrol.view.form.FormView

class UserFormView extends FormView
  html: require './template.html'

module.exports = (tag, inputConfigs)->
  class _UserFormView extends UserFormView
    tag: tag
    inputConfigs: inputConfigs

  new _UserFormView

  riot.mount tag

