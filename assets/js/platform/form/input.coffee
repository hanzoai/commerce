crowdcontrol = require 'crowdcontrol'

InputConfig = crowdcontrol.view.form.InputConfig

module.exports = (name, placeholder, hints)->
  return new InputConfig(name, '', placeholder, hints)
