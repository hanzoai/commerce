crowdcontrol = require 'crowdcontrol'

InputView = crowdcontrol.view.form.InputView

helpers = crowdcontrol.view.form.helpers
helpers.defaultTagName = 'basic-input'

# views
class BasicInputView extends InputView
  tag: 'basic-input'
  html: require './basic-form.html'

new BasicInputView

class SelectView extends InputView
  html: require './select.html'
  mixins:
    options: ()->
  events:
    update: ()->
      $(@root).find('select').chosen
        width: '100%'
        disable_search_threshold: 3

class CountriesSelectView extends SelectView
  tag: 'countries-select'
  mixins:
    options: ()->
      return window.countries

new CountriesSelectView

# tag registration
helpers.registerTag (inputCfg)->
  return inputCfg.hints.indexOf('countries') >= 0
, 'countries-select'

# validator registration
helpers.registerValidator ((inputCfg) -> return inputCfg.hints.indexOf('email') >= 0), (model, name)->
  value = model[name]
  throw new Error "Enter a valid email" if !value?

  value = value.trim().toLowerCase()
  re = /[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?/
  if value.match(re)
    return value
  throw new Error "Enter a valid email"

# module.exports =
#   BasicInputView: BasicInputView
