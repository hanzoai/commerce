# Validation helper
exports.isEmpty = (str) ->
  str.trim().length is 0

exports.isEmail = (email) ->
  pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i)
  pattern.test email

exports.error = (el) ->
  $(el).addClass 'error'
  $(el).addClass 'shake'
  setTimeout ->
    $(el).removeClass 'shake'
  , 500
