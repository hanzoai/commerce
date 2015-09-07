# tag/validator registration must occur first
require './controls'

module.exports =
  admin:            require './admin'
  generic:          require './generic'
  randomPassword:   require './random-password'
  pane:             require './pane'
