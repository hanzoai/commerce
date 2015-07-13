# tag/validator registration must occur first
require './controls'

module.exports =
  forms: require './forms'
  randomPassword: require './random-password'
