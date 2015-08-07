_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'

Api = crowdcontrol.data.Api

class OrganizationForm extends Form
  tag: 'organization-admin-form'
  path: 'organization'

  prefill: true

  inputConfigs: [
    input('name', '',  'static')
    input('fullName', 'Ex. Crowdstart',  'required')
    input('website', 'Ex. www.crowdstart.com', '')
    input('emailWhitelist', 'Ex. your@email.com', 'text')
    input('googleAnalytics', '')
    input('facebookTag', '')
  ]

OrganizationForm.register()

module.exports = OrganizationForm
