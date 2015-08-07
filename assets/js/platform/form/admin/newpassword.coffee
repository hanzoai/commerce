_ = require 'underscore'

input = require '../input'
Form = require './form'

class NewPasswordForm extends Form
  tag: 'new-password-admin-form'
  path: 'profile/password'

  prefill: false

  inputConfigs: [
    input('oldPassword', '******',      'password required min:6')
    input('password', '******',         'password required min:6')
    input('confirmPassword', '******',  'password required min:6 password-match:password')
  ]

NewPasswordForm.register()

module.exports = NewPasswordForm
