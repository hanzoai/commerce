# tag/validator registration must occur first
require './controls'

module.exports =
  generic: require './generic'
  randomPassword: require './random-password'
