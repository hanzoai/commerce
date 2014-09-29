#!/usr/bin/env coffee
csv = require 'csv'
db  = require './db'

opts =
  delimiter: '\t'
  escape: '"'

extensions =
  'mailcheckerplus':   0
  'scrolltotopbutton': 1
  'smoothgestures':    2
  'trollemoticons':    3
  'neatbookmarks':     4
  'mousestroke':       5
  'smoothscroll':      6
  'drag2up':           7
  'cloudsave':         8

getExtensionId = (s) ->
  for ext, id of extensions
    if (s.indexOf ext) != -1
      return id
  return null

exports.import = (path) ->
  parser = csv().from.path path, opts

  (parser.transform (row, index, cb) ->
    return cb() if index == 0

    [email, extension] = (c.toLowerCase().trim().replace(/\s+/g, '') for c in [row[9], row[14]])

    unless (id = getExtensionId extension)?
      return cb()

    db.previousPurchase email, id, (err) ->
      console.error err if err?
      cb()

  , parallel: 10).on 'end', ->
    db.end()

unless module.parent
  exports.import process.argv[2]
