_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class ApiKeysForm extends Form
  tag: 'api-keys-admin-form'
  path: 'keys'
  processButtonText: 'Generating'
  successButtonText: 'Generated'

  prefill: false

  events:
    "#{ Events.Form.ResponseSuccess }": ()->
      # need to reload to get new keys
      window.location.reload()

  inputConfigs: [
    input('live-secret-key', '', 'text')
    input('live-published-key', '', 'text')
    input('test-secret-key', '', 'text')
    input('test-published-key', '', 'text')
  ]

  js: ()->
    @model = window.Keys

    super

ApiKeysForm.register()

module.exports = ApiKeysForm
