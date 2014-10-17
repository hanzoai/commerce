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
        id      INT AUTO_INCREMENT PRIMARY KEY,
        email   VARCHAR(255) NOT NULL,
        name    VARCHAR(255) NOT NULL,
        address VARCHAR(255) NOT NULL,
        UNIQUE(email)
      );
    ''', (err) ->
      cb err if err?

      @query '''
        CREATE TABLE items (
          id       INT AUTO_INCREMENT PRIMARY KEY,
          name     VARCHAR(255) NOT NULL,
          price    FLOAT NOT NULL,
          sku      VARCHAR(40) NOT NULL,
          minimum  INT NOT NULL
        );
      ''', (err) ->
        cb err if err?
    
        @query '''
          CREATE TABLE line_items (
            id         INT AUTO_INCREMENT PRIMARY KEY,
            quantity   INT NOT NULL,
            user_id    INT NOT NULL,
            item_id    INT NOT NULL,
          
            FOREIGN KEY(user_id) REFERENCES users(id),
            FOREIGN KEY(item_id) REFERENCES items(id)
          );
        ''', (err) ->
          cb err
  
  dropdb: (cb = ->) ->
    @query 'DROP TABLE users', (err) ->
      cb err

  end: ->
    pool.end()
