_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
Form = require './form'

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

  generateModal: (event)->
    bootbox.dialog
      title: 'Warning: This will reset all your keys!'
      message: 'Any software integrated with Hanzo will need to be updated to use the new keys.'

      buttons:
        Reset:
          className: 'btn btn-danger'
          callback: ()=>
            @submit event

        Cancel:
          className: 'btn btn-primary'
          callback: ()->

  js: ()->
    @model = window.Keys

    super

ApiKeysForm.register()

module.exports = ApiKeysForm
