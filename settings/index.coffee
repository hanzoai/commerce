env = process.env.NODE_ENV ? 'development'

switch env
  when 'production'
    module.exports = require './production'
  else
    module.exports = require './development'
