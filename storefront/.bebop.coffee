#!/usr/bin/env coffee
fs        = require 'fs'
path      = require 'path'
requisite = require 'requisite'

module.exports =
  port: 3001

  compilers:
    jade: (src) ->
      if /index.jade$/.test src
        "node_modules/.bin/jade --pretty #{src} --out #{path.dirname src}"
      else
        "node_modules/.bin/requisite donate/js/storefront.coffee -o storefront.js"

    coffee: (src) ->
      "node_modules/.bin/requisite donate/js/storefront.coffee -o storefront.js"

    styl: (src) ->
      "node_modules/.bin/stylus donate/css/storefront.styl -o ."
