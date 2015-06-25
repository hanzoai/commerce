# tag/validator registration must occur first
require './controls'

module.exports =
  # must be after controls
  user: require './user'
