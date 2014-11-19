EventEmitter = require 'mvstar/lib/event-emitter'

class Cart extends EventEmitter
  constructor: (opts = {}) ->
    super
    @cookieName = app.get 'cookieName'
    @quantity = 0
    @subtotal = 0
    @cart = {}
    @fetch()

  fetch: ->
    cart = ($.cookie @cookieName) ? {}

    # grab stored quantity/subtotal
    unless isNaN cart.subtotal
      @subtotal = cart.subtotal

    unless isNaN cart.quantity
      @quantity = cart.quantity

    delete cart.quantity
    delete cart.subtotal

    @cart = cart

  save: (cart) ->
    @cart = cart if cart?
    @update()

    # persist quantity/subtotal too
    @cart.quantity = @quantity
    @cart.subtotal = @subtotal

    $.cookie @cookieName, @cart,
      expires: 30
      path: "/"

    delete @cart.quantity
    delete @cart.subtotal

  get: (sku) ->
    @cart[sku]

  set: (sku, item) ->
    @cart[sku] = item
    @save()
    item

  items: ->
    @cart

  add: (item) ->
    unless (_item = @get item.sku)?
      return @set item.sku, item

    _item.quantity += item.quantity
    @quantity += item.quantity
    @subtotal += item.quantity * item.price
    @emit 'quantity', @quantity
    @emit 'subtotal', @subtotal

    _item

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

module.exports = new Cart()
