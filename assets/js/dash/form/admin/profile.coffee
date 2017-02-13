_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
Form = require './form'

class ProfileForm extends Form
  tag: 'profile-admin-form'
  path: 'profile'

  prefill: true

  inputConfigs: [
    input('email', 'Email', 'required')
    input('firstName', 'First Name', 'required')
    input('lastName', 'Last Name', 'required')
    input('phone', 'Phone')
  ]

ProfileForm.register()

module.exports = ProfileForm
