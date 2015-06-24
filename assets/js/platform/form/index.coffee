module.exports =
  # tag/validator registration must occur first
  controls: require './controls'

  # must be after controls
  user: require './user'
