EventEmitter = require './event-emitter'

class Cart extends EventEmitter
  constructor: (opts = {}) ->
    super
    @cookieName = app.get 'cookieName'
    @fetch()

  fetch: ->
    @cart = ($.cookie @cookieName) ? {}
    @update()

  save: (cart) ->
    @cart = cart if cart?
    @update()

    $.cookie @cookieName, @cart,
      expires: 30
      path: "/"

  get: (sku) ->
    @cart[sku]

  set: (sku, item) ->
    @cart[sku] = item
    @save()

  add: (item) ->
    if (_item = @get item.sku)?
      _item.quantity += item.quantity
      @save()
    else
      @set item.sku, item

  remove: (sku, el) ->
    delete @cart[sku]
    @save()

  clear: ->
    @cart = {}
    @save()

  update: ->
    quantity = 0
    subtotal = 0

    for sku of @cart
      item = @cart[sku]
      quantity += item.quantity
      subtotal += item.price * item.quantity

    @quantity = quantity
    @subtotal = subtotal

    @emit 'quantity', quantity
    @emit 'subtotal', subtotal
    @emit 'update',   quantity, subtotal

module.exports = new Cart()
