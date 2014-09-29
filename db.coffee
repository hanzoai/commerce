mysql    = require 'mysql'
settings = require './settings'

pool = mysql.createPool settings.db

module.exports =
  query: (query, params = [], cb = ->) ->
    if typeof params == 'function'
      [params, cb] = [[], params]

    pool.getConnection (err, connection) ->
      return cb err if err?

      console.log query, if params.length > 0 then params else ''

      connection.query query, params, (err, rows, fields) ->
        console.log err.toString() if err?
        return cb err if err?

        (console.log rows.length + ' rows retrieved.') if rows?
        cb null, rows
        connection.release()

  checkPurchase: (id, email, cb) ->
    @query 'SELECT count(*) as purchased FROM users WHERE email = ? AND extensionId = ?', [email, id], (err, rows) ->
      cb null, rows[0].purchased > 0

  previousPurchase: (email, id, cb) ->
    @query 'INSERT INTO users (email, extensionId, previousDonation) VALUES (?, ?, true) ON DUPLICATE KEY UPDATE previousDonation=true', [email, id], cb

  savePurchase: (email, id, cb) ->
    @query 'INSERT INTO users (email, extensionId) VALUES (?, ?)', [email, id], cb

  removePurchase: (email, id, cb) ->
    @query 'DELETE FROM users where email = ? and extensionId = ?', [email, id], cb

  createdb: (cb = ->) ->
    @query '''
      CREATE TABLE users (
        email       VARCHAR(255) NOT NULL,
        extensionId VARCHAR(55)  NOT NULL,
        previousDonation BOOL,
        CONSTRAINT uc_email_extensionId UNIQUE (email, extensionId)
      )
    ''', (err) ->
      cb err

  dropdb: (cb = ->) ->
    @query 'DROP TABLE users', (err) ->
      cb err

  end: ->
    pool.end()
